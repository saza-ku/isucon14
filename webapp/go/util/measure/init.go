package measure

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	_ "net/http/pprof"

	"github.com/go-chi/chi/v5"
	"github.com/riandyrn/otelchi"
)

func PrepareMeasure(r *chi.Mux) {
	go func() {
		log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	}()

	_, err := initializeTracerProvider()
	if err != nil {
		panic(err)
	}

	r.Use(otelchi.Middleware("webapp", otelchi.WithChiRoutes(r)))
	r.Post("/setup", postSetup)
}

func CallSetup(port int) {
	go func() {
		for i := 1; i <= 1; i++ {
			r, err := http.Post(fmt.Sprintf("http://isucon%d:%d/setup", i, port), "application/json", nil)
			if err != nil {
				fmt.Printf("failed to call setup: isucon%d: %s\n", i, err)
			}

			if r.StatusCode != http.StatusOK {
				fmt.Printf("failed to call setup: isucon%d: status=%d\n", i, r.StatusCode)
			}
		}
	}()
}

func postSetup(w http.ResponseWriter, r *http.Request) {
	fmt.Println("====isucon-log-delimiter====")

	go func() {
		cmd := exec.Command("/home/isucon/scripts/measure.sh")
		_, err := cmd.Output()
		cmd.Stderr = os.Stderr
		if err != nil {
			fmt.Println("failed to run measure.sh")
			fmt.Println(err)
		}
	}()

	w.WriteHeader(http.StatusOK)
}
