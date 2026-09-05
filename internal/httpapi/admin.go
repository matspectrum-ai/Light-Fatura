package httpapi

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"github.com/matspectrum-ai/Claro-Fatura/internal/admin"
)

func (s *FullServer) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	_, ok := s.adminUser(w, r)
	return ok
}

func (s *FullServer) adminMetrics(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	value, err := s.admin.Dashboard(r.Context())
	if err != nil {
		adminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *FullServer) adminClearMetrics(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if err := s.admin.ClearMetrics(r.Context()); err != nil {
		adminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *FullServer) adminInvoices(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("pagina"))
	value, err := s.admin.ListInvoices(r.Context(), r.URL.Query().Get("busca"), page)
	if err != nil {
		adminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *FullServer) adminSaveInvoice(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var in admin.Invoice
	if decodeJSON(r, &in) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"erro": "Dados inválidos."})
		return
	}
	in.ID = r.PathValue("id")
	if err := s.admin.SaveInvoice(r.Context(), in); err != nil {
		adminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *FullServer) adminInvoiceStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if decodeJSON(r, &body) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"erro": "Dados inválidos."})
		return
	}
	if err := s.admin.SetInvoiceStatus(r.Context(), r.PathValue("id"), body.Status); err != nil {
		adminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *FullServer) adminDeleteAll(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var body struct {
		Confirmation string `json:"confirmacao"`
	}
	if decodeJSON(r, &body) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"erro": "Confirmação inválida."})
		return
	}
	if err := s.admin.DeleteAll(r.Context(), body.Confirmation); err != nil {
		adminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *FullServer) adminPayments(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	rows, err := s.admin.Payments(r.Context())
	if err != nil {
		adminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *FullServer) adminTransactions(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	rows, err := s.admin.Transactions(r.Context())
	if err != nil {
		adminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *FullServer) adminLogs(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	rows, err := s.admin.Logs(r.Context())
	if err != nil {
		adminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *FullServer) adminGateways(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	rows, err := s.admin.Gateways(r.Context())
	if err != nil {
		adminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *FullServer) adminCreateGateway(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var row admin.Gateway
	if decodeJSON(r, &row) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"erro": "Dados inválidos."})
		return
	}
	row.ID = ""
	if err := s.admin.CreateGateway(r.Context(), row); err != nil {
		adminError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *FullServer) adminSaveGateway(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var row admin.Gateway
	if decodeJSON(r, &row) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"erro": "Dados inválidos."})
		return
	}
	row.ID = r.PathValue("id")
	if err := s.admin.SaveGateway(r.Context(), row); err != nil {
		adminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *FullServer) adminDeleteGateway(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if err := s.admin.DeleteGateway(r.Context(), r.PathValue("id")); err != nil {
		adminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *FullServer) adminUseOnlyGateway(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if err := s.admin.UseOnlyGateway(r.Context(), r.PathValue("id")); err != nil {
		adminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *FullServer) adminWebhookSummaries(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	rows, err := s.admin.WebhookSummaries(r.Context())
	if err != nil {
		adminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *FullServer) adminRouting(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	value, err := s.admin.Routing(r.Context())
	if err != nil {
		adminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *FullServer) adminSaveRouting(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var row admin.Routing
	if decodeJSON(r, &row) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"erro": "Dados inválidos."})
		return
	}
	if err := s.admin.SaveRouting(r.Context(), row); err != nil {
		adminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *FullServer) adminImportCSV(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	reader, closeFn, err := csvBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"erro": err.Error()})
		return
	}
	if closeFn != nil {
		defer closeFn()
	}
	raw, err := io.ReadAll(io.LimitReader(reader, 16<<20))
	if err != nil || len(bytes.TrimSpace(raw)) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"erro": "CSV inválido ou vazio."})
		return
	}

	csvReader := csv.NewReader(bytes.NewReader(raw))
	csvReader.Comma = detectDelimiter(raw)
	csvReader.FieldsPerRecord = -1
	csvReader.TrimLeadingSpace = true
	records, err := csvReader.ReadAll()
	if err != nil || len(records) < 2 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"erro": "CSV inválido ou vazio."})
		return
	}

	headers := make(map[string]int)
	for i, h := range records[0] {
		headers[normalizeHeader(h)] = i
	}
	rows := make([]admin.ImportRow, 0, len(records)-1)
	for _, record := range records[1:] {
		if emptyRecord(record) {
			continue
		}
		get := func(names ...string) string {
			for _, name := range names {
				if idx, ok := headers[normalizeHeader(name)]; ok && idx < len(record) {
					return strings.TrimSpace(record[idx])
				}
			}
			return ""
		}

		original, ok := parseMoney(get("valor_original", "valor", "valor_taxa", "valor_fatura"))
		if !ok {
			original = 0
		}
		var discount *float64
		if text := get("valor_desconto", "desconto", "valor_com_desconto"); text != "" {
			if value, ok := parseMoney(text); ok {
				discount = &value
			}
		}
		metadata := map[string]string{
			"cte":           get("cte"),
			"instalacao":    get("instalacao", "instalação"),
			"endereco":      get("endereco", "endereço"),
			"endereco_rua":  get("endereco_rua", "rua"),
			"bairro":        get("bairro"),
			"cidade":        get("cidade"),
			"estado":        get("estado", "uf"),
			"cep":           get("cep"),
			"contrato":      get("contrato"),
			"conta_contrato": get("conta_contrato", "conta contrato"),
			"mes_ref":       get("mes_ref", "mes referencia", "mês referência"),
			"parcneg":       get("parcneg", "parcela"),
		}
		rows = append(rows, admin.ImportRow{
			Document: get("documento", "cpf", "codigo", "código"),
			Name: get("nome", "cliente", "nome_cliente"),
			Phone: get("telefone", "celular"),
			Email: get("email", "e-mail"),
			Original: original,
			Discount: discount,
			DueDate: normalizeDate(get("vencimento", "data_vencimento", "vence")),
			Status: normalizeStatus(get("status")),
			Metadata: metadata,
		})
	}

	result, err := s.admin.Import(r.Context(), rows)
	if err != nil {
		adminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func csvBody(r *http.Request) (io.Reader, func(), error) {
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(16 << 20); err != nil {
			return nil, nil, errors.New("não foi possível ler o formulário")
		}
		var file multipart.File
		var err error
		for _, name := range []string{"arquivo", "file", "csv"} {
			file, _, err = r.FormFile(name)
			if err == nil {
				break
			}
		}
		if err != nil {
			return nil, nil, errors.New("arquivo CSV ausente")
		}
		return file, func() { _ = file.Close() }, nil
	}
	return io.LimitReader(r.Body, 16<<20), nil, nil
}

func detectDelimiter(raw []byte) rune {
	text := strings.TrimPrefix(string(raw), "\ufeff")
	var header string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line != "" {
			header = line
			break
		}
	}
	if header == "" {
		return ';'
	}
	counts := map[rune]int{';': 0, ',': 0, '\t': 0}
	quoted := false
	for i, r := range header {
		if r == '"' {
			if quoted && i+1 < len(header) && header[i+1] == '"' {
				continue
			}
			quoted = !quoted
			continue
		}
		if !quoted {
			if _, ok := counts[r]; ok {
				counts[r]++
			}
		}
	}
	best := ';'
	for _, candidate := range []rune{',', '\t'} {
		if counts[candidate] > counts[best] {
			best = candidate
		}
	}
	return best
}

func emptyRecord(record []string) bool {
	for _, value := range record {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func normalizeHeader(value string) string {
	value = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(value), "\ufeff"))
	var b strings.Builder
	underscore := false
	for _, r := range value {
		switch r {
		case 'á', 'à', 'ã', 'â', 'ä':
			r = 'a'
		case 'é', 'è', 'ê', 'ë':
			r = 'e'
		case 'í', 'ì', 'î', 'ï':
			r = 'i'
		case 'ó', 'ò', 'õ', 'ô', 'ö':
			r = 'o'
		case 'ú', 'ù', 'û', 'ü':
			r = 'u'
		case 'ç':
			r = 'c'
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			underscore = false
		} else if b.Len() > 0 && !underscore {
			b.WriteByte('_')
			underscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

func parseMoney(value string) (float64, bool) {
	value = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(value, "R$", ""), " ", ""))
	if value == "" {
		return 0, false
	}
	if strings.Contains(value, ",") {
		value = strings.ReplaceAll(value, ".", "")
		value = strings.ReplaceAll(value, ",", ".")
	}
	number, err := strconv.ParseFloat(value, 64)
	return number, err == nil
}

func normalizeDate(value string) string {
	value = strings.TrimSpace(value)
	if len(value) == 10 && value[2] == '/' && value[5] == '/' {
		return value[6:10] + "-" + value[3:5] + "-" + value[:2]
	}
	return value
}

func normalizeStatus(value string) string {
	value = normalizeHeader(value)
	switch value {
	case "", "aberto", "em_aberto", "pendente":
		return "em_aberto"
	case "vencida", "vencido":
		return "vencida"
	case "paga", "pago", "confirmado":
		return "paga"
	case "cancelada", "cancelado":
		return "cancelada"
	case "expirada", "expirado":
		return "expirada"
	case "falhou", "falha":
		return "falhou"
	case "em_processamento", "processando":
		return "em_processamento"
	}
	return value
}

func adminError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, admin.ErrInvalid) {
		status = http.StatusBadRequest
	}
	message := "Erro interno."
	if status == http.StatusBadRequest {
		message = "Dados inválidos."
	}
	writeJSON(w, status, map[string]string{"erro": message})
	if status == http.StatusInternalServerError {
		fmt.Printf("admin error: %v\n", err)
	}
}
