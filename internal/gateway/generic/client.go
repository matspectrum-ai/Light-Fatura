package generic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
)

type Client struct { http *http.Client; lookup func(string) string }
func New() *Client { return &Client{http:&http.Client{Timeout:30*time.Second,Transport:&http.Transport{MaxIdleConns:128,MaxIdleConnsPerHost:64,IdleConnTimeout:90*time.Second}},lookup:os.Getenv} }
func (c *Client) Name() string { return "generico" }
func (c *Client) Configured(g gateway.Record) bool { if g.APIURL==nil||strings.TrimSpace(*g.APIURL)==""||len(g.SecretNames)==0{return false};for _,n:=range g.SecretNames{if c.lookup(n)==""{return false}};return true }
func (c *Client) CreatePIX(ctx context.Context,in gateway.CreateInput)(gateway.CreatedPIX,error){g:=in.Gateway;if g.APIURL==nil{return gateway.CreatedPIX{},errors.New("URL da API não configurada")};var token,secret string;if len(g.SecretNames)>0{token=c.lookup(g.SecretNames[0])};if len(g.SecretNames)>1{secret=c.lookup(g.SecretNames[1])};if token==""{return gateway.CreatedPIX{},errors.New("credencial não configurada")};email:=deref(in.Email);if email==""{base:=in.Phone;if base==""{base="cliente"};email=base+"@lightfatura.app"};body:=map[string]any{"amount":float64(in.AmountCents)/100,"amount_cents":in.AmountCents,"method_pay":"pix","payment_method":"pix","description":in.Description,"external_id":in.Reference,"postback":in.WebhookURL,"webhook_url":in.WebhookURL,"customer":map[string]any{"name":fallback(in.Name,"Cliente"),"email":email,"phone":in.Phone,"document":deref(in.Document)}};payload,_:=json.Marshal(body);req,e:=http.NewRequestWithContext(ctx,http.MethodPost,*g.APIURL,bytes.NewReader(payload));if e!=nil{return gateway.CreatedPIX{},e};req.Header.Set("Content-Type","application/json");req.Header.Set("Accept","application/json");req.Header.Set("Authorization","Bearer "+token);if secret!=""{req.Header.Set("x-secret-key",secret)};resp,e:=c.http.Do(req);if e!=nil{return gateway.CreatedPIX{},e};defer resp.Body.Close();raw,_:=io.ReadAll(io.LimitReader(resp.Body,1<<20));if resp.StatusCode<200||resp.StatusCode>=300{return gateway.CreatedPIX{},fmt.Errorf("HTTP %d",resp.StatusCode)};var data any;if json.Unmarshal(raw,&data)!=nil{return gateway.CreatedPIX{},errors.New("resposta inválida da gateway")};copyPaste:=extractCopyPaste(data,0);id:=search(data,[]string{"idTransaction","transaction_id","transactionId","id"},0);if copyPaste==""||id==""{return gateway.CreatedPIX{},errors.New("gateway não devolveu o código PIX")};qr:=search(data,[]string{"qrcode_image","qr_code_base64","qrCodeImage"},0);st:=search(data,[]string{"status"},0);if st==""{st="pendente"};exp:=search(data,[]string{"expires_at","expiration","expiraEm"},0);return gateway.CreatedPIX{TransactionID:id,CopyPaste:copyPaste,QRCode:ptr(qr),Status:st,ExpiresAt:ptr(exp)},nil}
func (c *Client) Status(context.Context,string,gateway.Record)(*string,error){return nil,nil}
func (c *Client) Paid(status *string)bool{if status==nil{return false};switch strings.ToLower(strings.TrimSpace(*status)){case "pago","paid","approved","completed","success","succeeded","done","confirmado","realizado","finalizado":return true;default:return false}}
func (c *Client) ReadWebhook(req *http.Request,raw []byte,g gateway.Record)(gateway.WebhookRead,error){var data any;if json.Unmarshal(raw,&data)!=nil{return gateway.WebhookRead{Valid:false},nil};var expected string;for _,n:=range g.SecretNames{if strings.Contains(strings.ToUpper(n),"WEBHOOK"){expected=c.lookup(n);break}};sent:=req.Header.Get("x-webhook-secret");if sent==""{sent=req.Header.Get("x-signature")};valid:=expected==""||sent==expected;return gateway.WebhookRead{Valid:valid,TransactionID:ptr(search(data,[]string{"idTransaction","transaction_id","transactionId","id"},0)),Status:ptr(search(data,[]string{"status","payment_status"},0)),Event:ptr(search(data,[]string{"event","type"},0))},nil}
func extractCopyPaste(v any,depth int)string{if depth>6||v==nil{return ""};switch x:=v.(type){case string:if strings.HasPrefix(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(x," ",""),"\n",""),"\r",""),"\t",""),"00020"){return strings.TrimSpace(x)};case []any:for _,i:=range x{if y:=extractCopyPaste(i,depth+1);y!=""{return y}};case map[string]any:for _,i:=range x{if y:=extractCopyPaste(i,depth+1);y!=""{return y}}};return ""}
func search(v any,keys []string,depth int)string{if depth>6||v==nil{return ""};m,ok:=v.(map[string]any);if !ok{return ""};for _,k:=range keys{if x:=m[k];x!=nil{switch x.(type){case map[string]any,[]any:default:return fmt.Sprint(x)}}};for _,x:=range m{if y:=search(x,keys,depth+1);y!=""{return y}};return ""}
func deref(v *string)string{if v==nil{return ""};return *v}
func fallback(v,f string)string{if v==""{return f};return v}
func ptr(v string)*string{if v==""{return nil};return &v}
