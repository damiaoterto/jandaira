package brain

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
	"github.com/xuri/excelize/v2"
)

const (
	chunkSize    = 1500 // characters per chunk
	chunkOverlap = 150  // overlap between consecutive chunks
)

// ExtractText reads the content of a document and returns the plain text.
// Supported extensions: .pdf, .docx, .txt, .csv, .xlsx.
// .doc (OLE2 binary format) is not supported and returns an error.
func ExtractText(filename string, data []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".pdf":
		return extractPDF(data)
	case ".docx":
		return extractDOCX(data)
	case ".txt":
		return string(data), nil
	case ".csv":
		return string(data), nil
	case ".xlsx":
		return extractXLSX(data)
	case ".doc":
		return "", fmt.Errorf("formato .doc (binário OLE2) não é suportado — converta para .docx e tente novamente")
	default:
		return "", fmt.Errorf("extensão não suportada: %q (aceitos: .pdf, .docx, .txt, .csv, .xlsx)", ext)
	}
}

// ChunkText splits text into overlapping chunks ready for embedding.
func ChunkText(text string) []string {
	text = strings.TrimSpace(text)
	if len(text) == 0 {
		return nil
	}
	if len(text) <= chunkSize {
		return []string{text}
	}

	var chunks []string
	start := 0
	for start < len(text) {
		end := start + chunkSize
		if end > len(text) {
			end = len(text)
		}
		chunks = append(chunks, text[start:end])
		if end == len(text) {
			break
		}
		start += chunkSize - chunkOverlap
	}
	return chunks
}

// extractPDF reads all text pages from a PDF file.
func extractPDF(data []byte) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("falha ao abrir PDF: %w", err)
	}

	var sb strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		sb.WriteString(text)
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// extractDOCX unzips a .docx file and extracts plain text from word/document.xml.
func extractDOCX(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("falha ao abrir DOCX como ZIP: %w", err)
	}

	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("falha ao abrir word/document.xml: %w", err)
		}
		defer rc.Close()

		xmlData, err := io.ReadAll(rc)
		if err != nil {
			return "", fmt.Errorf("falha ao ler word/document.xml: %w", err)
		}

		return stripXMLTags(xmlData), nil
	}

	return "", fmt.Errorf("arquivo word/document.xml não encontrado no DOCX")
}

// stripXMLTags extracts the plain text content from an XML document.
func stripXMLTags(xmlData []byte) string {
	decoder := xml.NewDecoder(bytes.NewReader(xmlData))
	var sb strings.Builder
	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		if t, ok := tok.(xml.CharData); ok {
			s := strings.TrimSpace(string(t))
			if s != "" {
				sb.WriteString(s)
				sb.WriteString(" ")
			}
		}
	}
	return strings.TrimSpace(sb.String())
}

// extractXLSX reads all sheets from an XLSX file and returns their contents as text.
func extractXLSX(data []byte) (string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("falha ao abrir XLSX: %w", err)
	}
	defer f.Close()

	var sb strings.Builder
	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("=== Planilha: %s ===\n", sheet))
		for _, row := range rows {
			sb.WriteString(strings.Join(row, "\t"))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
	return sb.String(), nil
}
