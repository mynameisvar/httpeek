package run

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"httpeek/internal/proxy"
	"httpeek/internal/storage"
	"httpeek/internal/ui"
)

func main() {
	dataDir := filepath.Join(".", "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatal(err)
	}

	store, err := storage.New(filepath.Join(dataDir, "httpeek.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	go func() {
		addr := "127.0.0.1:8080"
		h, caPath := proxy.New(store, dataDir)
		fmt.Println("Proxy listening on http://" + addr)
		fmt.Println("Root CA (for HTTPS MITM):", caPath)
		if err := http.ListenAndServe(addr, h); err != nil {
			log.Fatal(err)
		}
	}()

	uiAddr := "127.0.0.1:8081"
	fmt.Println("UI: http://" + uiAddr + "/ui")
	if err := http.ListenAndServe(uiAddr, ui.NewServer(store)); err != nil {
		log.Fatal(err)
	}
}
