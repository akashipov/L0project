package compress

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

type GzipWriter struct {
	OldW   http.ResponseWriter
	Writer *gzip.Writer
	Log    *zap.SugaredLogger
}

func (w GzipWriter) WriteHeader(statusCode int) {
	w.OldW.WriteHeader(statusCode)
}

func (w GzipWriter) Header() http.Header {
	return w.OldW.Header()
}

func (w GzipWriter) Write(b []byte) (int, error) {
	contentType := w.OldW.Header().Get("Content-Type")
	w.Log.Infof(" Content-Type of response: '%s'\n", contentType)
	w.Log.Infoln(" Started encoding...")
	w.OldW.Header().Set("Content-Encoding", "gzip")
	w.WriteHeader(http.StatusOK)
	return w.Writer.Write(b)
}

func GzipHandle(next http.Handler, log *zap.SugaredLogger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			log.Infof("Skip gzip... content-type = '%s'\n", r.Header.Get("Content-Type"))
			next.ServeHTTP(w, r)
			return
		}
		gz, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
		if err != nil {
			io.WriteString(w, err.Error())
			return
		}
		defer gz.Close()

		next.ServeHTTP(GzipWriter{OldW: w, Writer: gz, Log: log}, r)
	})
}
