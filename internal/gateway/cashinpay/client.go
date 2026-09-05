package cashinpay

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
)

const defaultBaseURL = "https://api.cashinpaybr.com/api/v1"

type Client struct {
	secretKey     string
	webhookSecret string
	baseURL       string
	http          *http.Client
	productName   string
	customerEmail string
}

func New(secretKey, webhookSecret, productName, customerEmail string) *Client {
	return &Client{
		secretKey: secretKey, webhookSecret: webhookSecret, baseURL: defaultBaseURL,
		http: &http.Client{Timeout:30*time.Second,Transport:&http.Transport{MaxIdleConns:128,MaxIdleConnsPerHost:64,IdleConnTimeout:90*time.Second}},
		productName:valueOr(productName,gateway.DefaultProductName),customerEmail:valueOr(customerEmail,gateway.DefaultCustomerEmail),
	}
}
func (c *Client) Name() string { return "cashinpay" }
func (c *Client) Configured(gateway.Record) bool { return strings.TrimSpace(c.secretKey)!="" }
func (c *Client) CreatePIX(ctx context.Context,in gateway.CreateInput)(gateway.CreatedPIX,error){if !c.Configured(in.Gateway){return gateway.CreatedPIX{},errors.New("CASHINPAY_SECRET_KEY não configurada")};amount:=float64(max(1,in.AmountCents))/100;document:=normalizeDocument(in.Document,in.Phone);body:=map[string]any{"amount":amount,"transaction_id":valueOr(in.Reference,fmt.Sprintf("TX%d",time.Now().UnixMilli())),"description":c.productName,"customer":map[string]any{"name":normalizeCustomerName(in.Name),"email":c.customerEmail,"phone":valueOr(digits(in.Phone),"11999999999"),"document":document}};if in.WebhookURL!=""{body["postbackUrl"]=in.WebhookURL};var lastErr error;for attempt:=0;attempt<4;attempt++{created,retry,err:=c.createOnce(ctx,body);if err==nil{return created,nil};lastErr=err;if !retry||attempt==3{break};select{case<-ctx.Done():return gateway.CreatedPIX{},ctx.Err();case<-time.After(time.Duration(400*(attempt+1))*time.Millisecond):}};if lastErr==nil{lastErr=errors.New("CashinPay não retornou a cobrança")};return gateway.CreatedPIX{},lastErr}
func (c *Client) createOnce(ctx context.Context,body map[string]any)(gateway.CreatedPIX,bool,error){payload,err:=json.Marshal(body);if err!=nil{return gateway.CreatedPIX{},false,err};req,err:=http.NewRequestWithContext(ctx,http.MethodPost,c.baseURL+"/transactions",bytes.NewReader(payload));if err!=nil{return gateway.CreatedPIX{},false,err};c.authorize(req);resp,err:=c.http.Do(req);if err!=nil{return gateway.CreatedPIX{},true,fmt.Errorf("cashinpay create: %w",err)};defer resp.Body.Close();raw,_:=io.ReadAll(io.LimitReader(resp.Body,1<<20));var envelope struct{Success bool `json:"success"`;Data json.RawMessage `json:"data"`;Error struct{Message string `json:"message"`} `json:"error"`};_=json.Unmarshal(raw,&envelope);if resp.StatusCode<200||resp.StatusCode>=300||envelope.Error.Message!=""{text:=strings.ToLower(string(raw)+" "+envelope.Error.Message);duplicate:=resp.StatusCode==http.StatusConflict||strings.Contains(text,"transacao ja existe")||strings.Contains(text,"duplicate_transaction_id");return gateway.CreatedPIX{},resp.StatusCode>=500||duplicate,fmt.Errorf("CashinPay HTTP %d",resp.StatusCode)};var decoded any;if len(envelope.Data)>0&&string(envelope.Data)!="null"{if err:=json.Unmarshal(envelope.Data,&decoded);err!=nil{return gateway.CreatedPIX{},false,errors.New("resposta inválida da CashinPay")}}else if err:=json.Unmarshal(raw,&decoded);err!=nil{return gateway.CreatedPIX{},false,errors.New("resposta inválida da CashinPay")};copyPaste:=firstField(decoded,[]string{"copy_paste","qrcode","qrCode","pixCode","copyPaste","emv","payload","brcode"});if copyPaste==""{return gateway.CreatedPIX{},false,errors.New("CashinPay não retornou código PIX")};id:=firstField(decoded,[]string{"id","transactionId","transaction_id"});qr:=firstField(decoded,[]string{"qrcode","qrCode","qr_code","pix_qrcode"});status:=firstField(decoded,[]string{"status"});if status==""{status="pending"};return gateway.CreatedPIX{TransactionID:id,CopyPaste:copyPaste,QRCode:ptrIf(qr),Status:status},false,nil}
func (c *Client) Status(ctx context.Context,id string,_ gateway.Record)(*string,error){req,err:=http.NewRequestWithContext(ctx,http.MethodGet,c.baseURL+"/transactions/"+id,nil);if err!=nil{return nil,err};c.authorize(req);resp,err:=c.http.Do(req);if err!=nil{return nil,err};defer resp.Body.Close();if resp.StatusCode<200||resp.StatusCode>=300{return nil,fmt.Errorf("CashinPay HTTP %d",resp.StatusCode)};var decoded any;if err:=json.NewDecoder(resp.Body).Decode(&decoded);err!=nil{return nil,err};status:=firstField(decoded,[]string{"status"});if status==""{return nil,nil};return &status,nil}
func (c *Client) Paid(status *string)bool{if status==nil{return false};switch strings.ToLower(strings.TrimSpace(*status)){case "paid","approved","completed","confirmed","pago","aprovado":return true;default:return false}}
func (c *Client) ReadWebhook(req *http.Request,raw []byte,_ gateway.Record)(gateway.WebhookRead,error){var body any;if err:=json.Unmarshal(raw,&body);err!=nil{return gateway.WebhookRead{Valid:false},nil};valid:=true;if c.webhookSecret!=""{signature:=req.Header.Get("X-CashinPay-Signature");if signature!=""{mac:=hmac.New(sha256.New,[]byte(c.webhookSecret));_,_=mac.Write(raw);expected:=fmt.Sprintf("%x",mac.Sum(nil));valid=subtle.ConstantTimeCompare([]byte(signature),[]byte(expected))==1}};id:=firstField(body,[]string{"id","transaction_id","transactionId","external_id"});status:=firstField(body,[]string{"status","payment_status"});event:=firstField(body,[]string{"event","type"});return gateway.WebhookRead{Valid:valid,TransactionID:ptrIf(id),Status:ptrIf(status),Event:ptrIf(event)},nil}
func (c *Client) authorize(req *http.Request){req.Header.Set("Content-Type","application/json");req.Header.Set("Accept","application/json");req.Header.Set("Authorization","Bearer "+c.secretKey)}
func firstField(value any,keys []string)string{return firstFieldDepth(value,keys,0)}
func firstFieldDepth(value any,keys []string,depth int)string{if depth>6||value==nil{return ""};m,ok:=value.(map[string]any);if !ok{return ""};for _,key:=range keys{if v,exists:=m[key];exists&&v!=nil{switch typed:=v.(type){case string:if typed!=""{return typed};case float64,bool,json.Number:return fmt.Sprint(typed)}}};for _,v:=range m{if found:=firstFieldDepth(v,keys,depth+1);found!=""{return found}};return ""}
func digits(value string)string{var b strings.Builder;for _,r:=range value{if r>='0'&&r<='9'{b.WriteRune(r)}};return b.String()}
func normalizeCustomerName(value string)string{value=strings.TrimSpace(value);if value==""{return "Cliente"};return value}
func normalizeDocument(document *string,phone string)string{var value string;if document!=nil{value=digits(*document)};if len(value)==14||(len(value)==11&&validCPF(value)){return value};seed:=digits(phone);var hash int64=7;if seed==""{seed="0"};for _,ch:=range seed{hash=(hash*31+int64(ch))%1000000007};base:=fmt.Sprintf("%09d",hash%1000000000);if allSame(base){base="123456789"};return cpfWithDigits(base)}
func cpfWithDigits(base string)string{digitsV:=make([]int,0,11);for _,r:=range base{digitsV=append(digitsV,int(r-'0'))};check:=func(values []int)int{weight:=len(values)+1;sum:=0;for i,d:=range values{sum+=d*(weight-i)};r:=(sum*10)%11;if r==10{return 0};return r};d1:=check(digitsV);d2:=check(append(append([]int(nil),digitsV...),d1));return fmt.Sprintf("%s%d%d",base,d1,d2)}
func validCPF(value string)bool{return len(value)==11&&!allSame(value)&&cpfWithDigits(value[:9])==value}
func allSame(value string)bool{if value==""{return false};for i:=1;i<len(value);i++{if value[i]!=value[0]{return false}};return true}
func valueOr(value,fallback string)string{if strings.TrimSpace(value)==""{return fallback};return value}
func ptrIf(value string)*string{if value==""{return nil};return &value}
