package staticpix

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
)

type Client struct{ key, receiver, city string }

func New(key, receiver, city string) *Client {
	if receiver == "" {
		receiver = "LIGHT SERVICOS"
	}
	if city == "" {
		city = "RIO DE JANEIRO"
	}
	return &Client{key: key, receiver: receiver, city: city}
}

func (c *Client) Name() string { return "pix-estatico" }
func (c *Client) Configured(gateway.Record) bool { return strings.TrimSpace(c.key) != "" }
func (c *Client) CreatePIX(_ context.Context, in gateway.CreateInput) (gateway.CreatedPIX, error) {
	if !c.Configured(in.Gateway) {
		return gateway.CreatedPIX{}, fmt.Errorf("PIX_CHAVE não configurada")
	}
	txid := newTxID()
	code := brCode(c.key, float64(in.AmountCents)/100, c.receiver, c.city, txid)
	return gateway.CreatedPIX{TransactionID: txid, CopyPaste: code, Status: "pendente"}, nil
}
func (c *Client) Status(context.Context, string, gateway.Record) (*string, error) { return nil, nil }
func (c *Client) Paid(status *string) bool {
	if status == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(*status)) {
	case "pago", "paid", "approved", "completed", "success", "succeeded", "done", "confirmado", "realizado", "finalizado":
		return true
	default:
		return false
	}
}
func (c *Client) ReadWebhook(*http.Request, []byte, gateway.Record) (gateway.WebhookRead, error) {
	return gateway.WebhookRead{Valid: false}, nil
}
func tlv(id, value string) string { return id + fmt.Sprintf("%02d", len(value)) + value }
func crc16(payload string) string {
	crc := 0xffff
	for i := 0; i < len(payload); i++ {
		crc ^= int(payload[i]) << 8
		for j := 0; j < 8; j++ {
			if crc&0x8000 != 0 {
				crc = ((crc << 1) ^ 0x1021) & 0xffff
			} else {
				crc = (crc << 1) & 0xffff
			}
		}
	}
	return fmt.Sprintf("%04X", crc)
}
func clean(s string, max int) string {
	replacer := strings.NewReplacer(
		"Á", "A", "À", "A", "Â", "A", "Ã", "A", "Ä", "A", "á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
		"É", "E", "È", "E", "Ê", "E", "Ë", "E", "é", "e", "è", "e", "ê", "e", "ë", "e",
		"Í", "I", "Ì", "I", "Î", "I", "Ï", "I", "í", "i", "ì", "i", "î", "i", "ï", "i",
		"Ó", "O", "Ò", "O", "Ô", "O", "Õ", "O", "Ö", "O", "ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
		"Ú", "U", "Ù", "U", "Û", "U", "Ü", "U", "ú", "u", "ù", "u", "û", "u", "ü", "u", "Ç", "C", "ç", "c",
	)
	s = replacer.Replace(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' ' {
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(strings.ToUpper(b.String()))
	runes := []rune(out)
	if len(runes) > max {
		runes = runes[:max]
	}
	return string(runes)
}
func brCode(key string, value float64, name, city, txid string) string {
	merchant := tlv("00", "br.gov.bcb.pix") + tlv("01", key)
	n := clean(name, 25)
	if n == "" {
		n = "RECEBEDOR"
	}
	ct := clean(city, 15)
	if ct == "" {
		ct = "RIO DE JANEIRO"
	}
	tx := clean(txid, 25)
	if tx == "" {
		tx = "***"
	}
	without := tlv("00", "01") + tlv("01", "12") + tlv("26", merchant) + tlv("52", "0000") + tlv("53", "986") + tlv("54", fmt.Sprintf("%.2f", value)) + tlv("58", "BR") + tlv("59", n) + tlv("60", ct) + tlv("62", tlv("05", tx)) + "6304"
	return without + crc16(without)
}
func newTxID() string {
	var raw [5]byte
	_, _ = rand.Read(raw[:])
	n := time.Now().UnixMilli()
	suffix := strconv.FormatUint(uint64(raw[0])<<32|uint64(raw[1])<<24|uint64(raw[2])<<16|uint64(raw[3])<<8|uint64(raw[4]), 36)
	suffix = strings.ToUpper(suffix)
	if utf8.RuneCountInString(suffix) > 6 {
		suffix = suffix[:6]
	}
	return "PIX" + strings.ToUpper(strconv.FormatInt(n, 36)) + suffix
}
