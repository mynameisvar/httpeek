package run

import (
	"fmt"
	"httpeek/internal/proxy"
	"log"
	"net/http"
)

func main() {
	addr := ":8080"

	handler := proxy.NewProxy()
	fmt.Println("httpeek running on http://localhost" + addr)
	fmt.Println("Configure your client to use this as HTTP proxy.")

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}
