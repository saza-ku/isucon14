package measure

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	_ "net/http/pprof"

	"github.com/felixge/fgprof"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
)

func PrepareMeasure(e *echo.Echo) {
	http.DefaultServeMux.Handle("/debug/fgprof", fgprof.Handler())
	go func() {
		log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	}()

	_, err := initializeTracerProvider()
	if err != nil {
		panic(err)
	}

	e.Use(otelecho.Middleware("webapp"))
	e.POST("/setup", postSetup)
}

func CallSetup(port int) {
	go func() {
		for i := 1; i <= 3; i++ {
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

func postSetup(c echo.Context) error {
	fmt.Println("====isucon-log-delimiter====")

	go func() {
		cmd := exec.Command("/home/isucon/scripts/measure.sh")
		bytes, err := cmd.Output()
		cmd.Stderr = os.Stderr
		if err != nil {
			c.Logger().Errorf("exec measure.sh error: %v", err)
			c.Logger().Errorf("output: %v", string(bytes))
		}
	}()

	return c.NoContent(http.StatusOK)
}
