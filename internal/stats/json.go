package stats

import (
	"bufio"
	"encoding/json"
	"io"

	"github.com/ptitlu42/highrise-code/internal/scan"
)

func Write_json(w io.Writer, rep *Report, res *scan.Result) error {
	bw := bufio.NewWriterSize(w, 1<<16)
	enc := json.NewEncoder(bw)
	enc.SetEscapeHTML(false)
	if _, err := bw.WriteString("{\"report\":"); err != nil {
		return err
	}
	if err := enc.Encode(rep); err != nil {
		return err
	}
	type warning struct {
		Path    string `json:"path"`
		Message string `json:"message"`
	}
	warnings := make([]warning, 0, len(res.Warnings))
	for _, w := range res.Warnings {
		warnings = append(warnings, warning{Path: w.Path, Message: w.Message})
	}
	if _, err := bw.WriteString(",\"warnings\":"); err != nil {
		return err
	}
	if err := enc.Encode(warnings); err != nil {
		return err
	}
	if _, err := bw.WriteString(",\"files\":["); err != nil {
		return err
	}
	first := true
	err := Each_file(res, func(f File_stats) error {
		if !first {
			if err := bw.WriteByte(','); err != nil {
				return err
			}
		}
		first = false
		return enc.Encode(f)
	})
	if err != nil {
		return err
	}
	if _, err := bw.WriteString("]}\n"); err != nil {
		return err
	}
	return bw.Flush()
}
