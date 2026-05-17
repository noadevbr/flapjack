package build

import (
	"fmt"
	"net"
	"net/http"
)

func (b *Build) StartVisualServer(html string) (int, error) {
	port := 3248

	for {
		addr := fmt.Sprintf(":%d", port)

		ln, err := net.Listen("tcp", addr)
		if err != nil {
			port++
			continue
		}
		_ = ln.Close()

		go func(p int) {
			mux := http.NewServeMux()

			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				fmt.Fprint(w, renderHTML(html))
			})

			fmt.Printf("\n🌐 Flapjack Visualizer: http://localhost:%d\n", p)

			http.ListenAndServe(fmt.Sprintf(":%d", p), mux)
		}(port)

		return port, nil
	}
}
