package staticpix

import (
	"testing"

	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
)

func TestBRCodeCharacterization(t *testing.T) {
	got := brCode("11999999999", 12.34, "JOÃO DA SILVA", "SÃO PAULO", "ABC123")
	want := "00020101021226330014br.gov.bcb.pix011111999999999520400005303986540512.345802BR5913JOAO DA SILVA6009SAO PAULO62100506ABC1236304A35A"
	if got != want {
		t.Fatalf("unexpected BR Code\n got: %s\nwant: %s", got, want)
	}
}

func TestClientRequiresPIXKey(t *testing.T) {
	client := New("", "LIGHT SERVICOS", "RIO DE JANEIRO")
	if client.Configured(gateway.Record{}) {
		t.Fatal("client should not be configured without PIX key")
	}
}
