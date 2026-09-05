package httpapi

import "testing"

func TestDetectDelimiter(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want rune
	}{
		{"semicolon", "cpf;nome;valor_original\n123;Ana;100,00\n", ';'},
		{"comma", "cpf,nome,valor_original\n123,Ana,100.00\n", ','},
		{"tab", "cpf\tnome\tvalor_original\n123\tAna\t100\n", '\t'},
		{"bom", "\ufeffcpf;nome;valor_original\n123;Ana;100\n", ';'},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := detectDelimiter([]byte(test.raw)); got != test.want {
				t.Fatalf("detectDelimiter() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestNormalizeHeader(t *testing.T) {
	tests := map[string]string{
		"\ufeffCPF":          "cpf",
		"Mês Referência":     "mes_referencia",
		"Conta Contrato":     "conta_contrato",
		"VALOR COM DESCONTO": "valor_com_desconto",
		"Instalação":         "instalacao",
	}
	for input, want := range tests {
		if got := normalizeHeader(input); got != want {
			t.Fatalf("normalizeHeader(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParseMoney(t *testing.T) {
	for input, want := range map[string]float64{
		"R$ 1.234,56": 1234.56,
		"70,00":       70,
		"70.50":       70.50,
	} {
		got, ok := parseMoney(input)
		if !ok || got != want {
			t.Fatalf("parseMoney(%q) = %v, %v; want %v, true", input, got, ok, want)
		}
	}
	if _, ok := parseMoney(""); ok {
		t.Fatal("empty money value should be invalid")
	}
}

func TestNormalizeDateAndStatus(t *testing.T) {
	if got := normalizeDate("10/09/2026"); got != "2026-09-10" {
		t.Fatalf("normalizeDate = %q", got)
	}
	if got := normalizeStatus("Em processamento"); got != "em_processamento" {
		t.Fatalf("normalizeStatus = %q", got)
	}
	if got := normalizeStatus("PAGO"); got != "paga" {
		t.Fatalf("normalizeStatus = %q", got)
	}
}
