package main

import (
	"context"
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-sql-driver/mysql"
	"github.com/isucon/isucon14/webapp/go/util"
	"github.com/isucon/isucon14/webapp/go/util/measure"
	"github.com/jmoiron/sqlx"
)

var db *sqlx.DB
var distanceWorker *util.Worker[string]

const distanceWorkerInterval = 2500 * time.Millisecond

func main() {
	mux := setup()
	slog.Info("Listening on :8080")
	http.ListenAndServe(":8080", mux)
}

func setup() http.Handler {
	host := os.Getenv("ISUCON_DB_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := os.Getenv("ISUCON_DB_PORT")
	if port == "" {
		port = "3306"
	}
	_, err := strconv.Atoi(port)
	if err != nil {
		panic(fmt.Sprintf("failed to convert DB port number from ISUCON_DB_PORT environment variable into int: %v", err))
	}
	user := os.Getenv("ISUCON_DB_USER")
	if user == "" {
		user = "isucon"
	}
	password := os.Getenv("ISUCON_DB_PASSWORD")
	if password == "" {
		password = "isucon"
	}
	dbname := os.Getenv("ISUCON_DB_NAME")
	if dbname == "" {
		dbname = "isuride"
	}

	dbConfig := mysql.NewConfig()
	dbConfig.User = user
	dbConfig.Passwd = password
	dbConfig.Addr = net.JoinHostPort(host, port)
	dbConfig.Net = "tcp"
	dbConfig.DBName = dbname
	dbConfig.ParseTime = true

	_db, err := measure.NewIsuconDB(dbConfig)
	if err != nil {
		panic(err)
	}
	db = _db

	fmt.Println("initialize: distanceWorker: start")
	distanceWorker = util.NewWorker[string](distanceWorkerInterval)
	go distanceWorker.Run(distanceWorkerRunFunc)
	fmt.Println("initialize: distanceWorker: running")

	mux := chi.NewRouter()
	mux.Use(middleware.Logger)
	mux.Use(middleware.Recoverer)
	measure.PrepareMeasure(mux)
	mux.HandleFunc("POST /api/initialize", postInitialize)

	// app handlers
	{
		mux.HandleFunc("POST /api/app/users", appPostUsers)

		authedMux := mux.With(appAuthMiddleware)
		authedMux.HandleFunc("POST /api/app/payment-methods", appPostPaymentMethods)
		authedMux.HandleFunc("GET /api/app/rides", appGetRides)
		authedMux.HandleFunc("POST /api/app/rides", appPostRides)
		authedMux.HandleFunc("POST /api/app/rides/estimated-fare", appPostRidesEstimatedFare)
		authedMux.HandleFunc("POST /api/app/rides/{ride_id}/evaluation", appPostRideEvaluatation)
		authedMux.HandleFunc("GET /api/app/notification", appGetNotification)
		authedMux.HandleFunc("GET /api/app/nearby-chairs", appGetNearbyChairs)
	}

	// owner handlers
	{
		mux.HandleFunc("POST /api/owner/owners", ownerPostOwners)

		authedMux := mux.With(ownerAuthMiddleware)
		authedMux.HandleFunc("GET /api/owner/sales", ownerGetSales)
		authedMux.HandleFunc("GET /api/owner/chairs", ownerGetChairs)
	}

	// chair handlers
	{
		mux.HandleFunc("POST /api/chair/chairs", chairPostChairs)

		authedMux := mux.With(chairAuthMiddleware)
		authedMux.HandleFunc("POST /api/chair/activity", chairPostActivity)
		authedMux.HandleFunc("POST /api/chair/coordinate", chairPostCoordinate)
		authedMux.HandleFunc("GET /api/chair/notification", chairGetNotification)
		authedMux.HandleFunc("POST /api/chair/rides/{ride_id}/status", chairPostRideStatus)
	}

	// internal handlers
	{
		mux.HandleFunc("GET /api/internal/matching", internalGetMatching)
	}

	return mux
}

type postInitializeRequest struct {
	PaymentServer string `json:"payment_server"`
}

type postInitializeResponse struct {
	Language string `json:"language"`
}

func postInitialize(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req := &postInitializeRequest{}
	if err := bindJSON(r, req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	fmt.Printf("postInitialize: %v\n", req)
	distanceWorker.Close()
	fmt.Println("initialize: distanceWorker: closed")
	distanceWorker = util.NewWorker[string](distanceWorkerInterval)
	go distanceWorker.Run(distanceWorkerRunFunc)
	fmt.Println("initialize: distanceWorker: running")

	if out, err := exec.Command("../sql/init.sh").CombinedOutput(); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to initialize: %s: %w", string(out), err))
		return
	}

	if _, err := db.ExecContext(ctx, "UPDATE settings SET value = ? WHERE name = 'payment_gateway_url'", req.PaymentServer); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	queries := []string{
		"ALTER TABLE chairs ADD INDEX owner_id_idx (owner_id)",
		"CREATE INDEX idx_ride_statuses_ride_id_created_at ON ride_statuses (ride_id ASC, created_at DESC)",
		"CREATE INDEX idx_ride_statuses_ride_id_chair_sent_at_created_at ON ride_statuses (ride_id, chair_sent_at, created_at)",
		"CREATE INDEX idx_ride_statuses_ride_id_app_sent_at_created_at ON ride_statuses (ride_id, app_sent_at, created_at)",
		"CREATE INDEX chair_locations_chair_id_created_at ON chair_locations (chair_id ASC, created_at DESC)",
		"CREATE INDEX idx_rides_chair_id_updated_at ON rides (chair_id ASC, updated_at DESC)",
		"CREATE INDEX idx_chairs_access_token ON chairs (access_token)",
	}
	for _, q := range queries {
		if err := util.CreateIndexIfNotExists(db, q); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}

	chairIDs := []string{}
	if err := db.SelectContext(ctx, &chairIDs, "SELECT DISTINCT chair_id FROM chair_locations"); err != nil {
		writeError(w, http.StatusInternalServerError, err)
	}
	distanceWorkerRunFunc(chairIDs)

	measure.CallSetup(8080)

	writeJSON(w, http.StatusOK, postInitializeResponse{Language: "go"})
}

type Coordinate struct {
	Latitude  int `json:"latitude"`
	Longitude int `json:"longitude"`
}

type Distance struct {
	ChairID                string    `db:"chair_id"`
	TotalDistance          int       `db:"total_distance"`
	TotalDistanceUpdatedAt time.Time `db:"total_distance_updated_at"`
}

func bindJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func writeJSON(w http.ResponseWriter, statusCode int, v interface{}) {
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	buf, err := json.Marshal(v)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(statusCode)
	w.Write(buf)
}

func writeError(w http.ResponseWriter, statusCode int, err error) {
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(statusCode)
	buf, marshalError := json.Marshal(map[string]string{"message": err.Error()})
	if marshalError != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"marshaling error failed"}`))
		return
	}
	w.Write(buf)

	slog.Error("error response wrote", err)
}

func secureRandomStr(b int) string {
	k := make([]byte, b)
	if _, err := crand.Read(k); err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", k)
}

func distanceWorkerRunFunc(chairIDs []string) {
	fmt.Printf("distanceWorkerRunFunc: %v\n", chairIDs)
	ctx := context.Background()
	distances := make([]Distance, 0, len(chairIDs))
	query := `
		SELECT chair_id,
				SUM(IFNULL(distance, 0)) AS total_distance,
				MAX(created_at)          AS total_distance_updated_at
		FROM (SELECT chair_id,
					created_at,
					ABS(latitude - LAG(latitude) OVER (PARTITION BY chair_id ORDER BY created_at)) +
					ABS(longitude - LAG(longitude) OVER (PARTITION BY chair_id ORDER BY created_at)) AS distance
				FROM chair_locations) tmp
		WHERE chair_id IN (?)
		GROUP BY chair_id
	`
	query, params, err := sqlx.In(query, chairIDs)
	if err != nil {
		slog.Error("failed to update total_distance: %s", "error", err)
		return
	}
	tx, err := db.Beginx()
	defer tx.Rollback()
	if err := tx.SelectContext(ctx, &distances, query, params...); err != nil {
		slog.Error("failed to update total_distance", "error", err)
		return
	}

	upsertQuery := `
		INSERT INTO distances (chair_id, total_distance, total_distance_updated_at)
		VALUES (:chair_id, :total_distance, :total_distance_updated_at)
		ON DUPLICATE KEY UPDATE
			total_distance = VALUES(total_distance),
			total_distance_updated_at = VALUES(total_distance_updated_at)
	`
	if _, err := tx.NamedExecContext(ctx, upsertQuery, distances); err != nil {
		slog.Error("failed to upsert distances", "error", err)
		return
	}

	tx.Commit()
}
