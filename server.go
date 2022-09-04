package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"text/template"
	"time"

	"github.com/google/uuid"
)

type key int

const REQUEST_ID_KEY key = 0

var addr *string
var dev *bool

func init() {
	addr = flag.String("addr", "", "server address")
	dev = flag.Bool("dev", false, "dev environment (simplifies logging)")
}

func wrapPrice(s string) string {
	re := regexp.MustCompile(`(\$[\d,]+)`)
	return re.ReplaceAllString(s, `<span class="u-green">$1</span>`)
}

func render(w http.ResponseWriter, r *http.Request, name string, data any) {
	ctx := r.Context()
	logger := getLogger(ctx)

	funcMap := template.FuncMap{
		"wrapPrice": wrapPrice,
	}

	f := filepath.Join("templates", name)
	t, err := template.New(name).Funcs(funcMap).ParseFiles(f)
	if err != nil {
		logger.Printf("ParseFiles(templates/%s): %v", name, err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "Internal Server Error")
		return
	}

	err = t.Execute(w, data)
	if err != nil {
		logger.Printf("Execute(): %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "Internal Server Error")
	}
}

func getRequestID(ctx context.Context) string {
	id, ok := ctx.Value(REQUEST_ID_KEY).(string)
	if !ok {
		return "unknown"
	}
	return id
}

func getLogger(ctx context.Context) *log.Logger {
	id := getRequestID(ctx)
	if *dev {
		return log.New(os.Stdout, "", log.Lshortfile)
	}
	return log.New(os.Stdout, fmt.Sprintf("[%s]", id), log.LstdFlags)
}

func tracing(uuid func() string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-Id")
			if id == "" {
				id = uuid()
			}

			ctx := context.WithValue(r.Context(), REQUEST_ID_KEY, id)
			w.Header().Set("X-Request-Id", id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func logging(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rid := getRequestID(r.Context())
				if *dev {
					logger.Println(r.Method, r.URL.Path)
				} else {
					logger.Println(rid, r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent())
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags)

	flag.Parse()
	if *addr == "" {
		flag.Usage()
		return
	}
	logger.Println("starting")

	router := http.NewServeMux()
	router.HandleFunc("/", index)

	static := http.FileServer(http.Dir("static"))
	router.Handle("/static/", http.StripPrefix("/static/", static))

	s := &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  15 * time.Second,
		Addr:         *addr,
		Handler:      tracing(uuid.NewString)(logging(logger)(router)),
	}

	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	go func() {
		<-quit
		logger.Println("shutting down")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		s.SetKeepAlivesEnabled(false)

		if err := s.Shutdown(ctx); err != nil {
			logger.Fatalf("could not gracefully shutdown: %v", err)
		}
		close(done)
	}()

	logger.Println("ready at", *addr)
	if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("could not listen on %s: %v", *addr, err)
	}

	<-done
	logger.Println("goodbye")
}
