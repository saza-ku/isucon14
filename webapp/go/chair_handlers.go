package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/oklog/ulid/v2"
)

type chairPostChairsRequest struct {
	Name               string `json:"name"`
	Model              string `json:"model"`
	ChairRegisterToken string `json:"chair_register_token"`
}

type chairPostChairsResponse struct {
	ID      string `json:"id"`
	OwnerID string `json:"owner_id"`
}

func chairPostChairs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req := &chairPostChairsRequest{}
	if err := bindJSON(r, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Model == "" || req.ChairRegisterToken == "" {
		writeError(w, http.StatusBadRequest, errors.New("some of required fields(name, model, chair_register_token) are empty"))
		return
	}

	owner := &Owner{}
	if err := db.GetContext(ctx, owner, "SELECT * FROM owners WHERE chair_register_token = ?", req.ChairRegisterToken); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, errors.New("invalid chair_register_token"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	chairID := ulid.Make().String()
	accessToken := secureRandomStr(32)

	_, err := db.ExecContext(
		ctx,
		"INSERT INTO chairs (id, owner_id, name, model, is_active, access_token) VALUES (?, ?, ?, ?, ?, ?)",
		chairID, owner.ID, req.Name, req.Model, false, accessToken,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Path:  "/",
		Name:  "chair_session",
		Value: accessToken,
	})

	writeJSON(w, http.StatusCreated, &chairPostChairsResponse{
		ID:      chairID,
		OwnerID: owner.ID,
	})
}

type postChairActivityRequest struct {
	IsActive bool `json:"is_active"`
}

func chairPostActivity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	chair := ctx.Value("chair").(*Chair)

	req := &postChairActivityRequest{}
	if err := bindJSON(r, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	_, err := db.ExecContext(ctx, "UPDATE chairs SET is_active = ? WHERE id = ?", req.IsActive, chair.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type chairPostCoordinateResponse struct {
	RecordedAt int64 `json:"recorded_at"`
}

func chairPostCoordinate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req := &Coordinate{}
	if err := bindJSON(r, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	chair := ctx.Value("chair").(*Chair)
	chairLocationID := ulid.Make().String()

	tx, err := db.Beginx()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO chair_locations (id, chair_id, latitude, longitude) VALUES (?, ?, ?, ?)`,
		chairLocationID, chair.ID, req.Latitude, req.Longitude,
	); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	location := &ChairLocation{}
	if err := tx.GetContext(ctx, location, `SELECT * FROM chair_locations WHERE id = ?`, chairLocationID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	chairPostCoordinateWorker.Send(chairPostCoordinateItem{
		chair:           *chair,
		coordinate:      *req,
	})
	distanceWorker.Send(chair.ID)

	writeJSON(w, http.StatusOK, &chairPostCoordinateResponse{
		RecordedAt: location.CreatedAt.UnixMilli(),
	})
}

type chairPostCoordinateItem struct {
	chair           Chair
	coordinate      Coordinate
}

func chairPostCoordinateWorkerRunFunc(items []chairPostCoordinateItem) {
	ctx := context.Background()

	fmt.Printf("chairPostCoordinateWorkerRunFunc: items: %#v\n", items)
	tx, err := db.Beginx()
	if err != nil {
		slog.Error("chairPostCoordinate: failed to BeginTx", "error", err)
		return
	}
	defer tx.Rollback()

	chairIDs := make([]string, 0, len(items))
	itemMap := make(map[string]chairPostCoordinateItem)
	for _, item := range items {
		chairIDs = append(chairIDs, item.chair.ID)
		itemMap[item.chair.ID] = item
	}
	fmt.Printf("chairPostCoordinateWorkerRunFunc: chairIDs: %#v\n", chairIDs)

	query := "SELECT id, user_id, chair_id, pickup_latitude, pickup_longitude, destination_latitude, destination_longitude, evaluation, created_at, updated_at FROM (SELECT id, user_id, chair_id, pickup_latitude, pickup_longitude, destination_latitude, destination_longitude, evaluation, created_at, updated_at, ROW_NUMBER() OVER (PARTITION BY chair_id ORDER BY updated_at DESC) AS row_num FROM rides WHERE chair_id IN (?)) AS t WHERE row_num=1"
	query, args, err := sqlx.In(query, chairIDs)
	if err != nil {
		slog.Error("chairPostCoordinate: failed to Build query", "error", err)
		return
	}
	rides := []Ride{}
	if err := tx.SelectContext(ctx, &rides, query, args...); err != nil {
		slog.Error("chairPostCoordinate: failed to Get rides", "error", err)
		return
	}

	rideIDs := make([]string, 0, len(rides))
	rideMaps := make(map[string]Ride)
	for _, ride := range rides {
		rideIDs = append(rideIDs, ride.ID)
		rideMaps[ride.ID] = ride
	}
	fmt.Printf("chairPostCoordinateWorkerRunFunc: rideIDs: %#v\n", rideIDs)

	statuses, err := getLatestRideStatuses(ctx, tx, rideIDs, false)
	if err != nil {
		slog.Error("chairPostCoordinate: failed to Get statuses", "error", err)
		return
	}
	fmt.Printf("chairPostCoordinateWorkerRunFunc: statuses: %#v\n", statuses)

	insertStatuses := make([]RideStatus, 0, len(rides))
	for rideID, status := range statuses {
		ride := rideMaps[rideID]
		item := itemMap[ride.ChairID.String]
		fmt.Printf("chairPostCoordinateWorkerRunFunc: rideID: %s, status: %s, ride: %#v, item: %#v\n", rideID, status, ride, item)
		if status != "COMPLETED" && status != "CANCELED" {
			if item.coordinate.Latitude == ride.PickupLatitude && item.coordinate.Longitude == ride.PickupLongitude && status == "ENROUTE" {
				insertStatuses = append(insertStatuses, RideStatus{
					ID:     ulid.Make().String(),
					RideID: rideID,
					Status: "PICKUP",
				})
			}
			if item.coordinate.Latitude == ride.DestinationLatitude && item.coordinate.Longitude == ride.DestinationLongitude && status == "CARRYING" {
				insertStatuses = append(insertStatuses, RideStatus{
					ID:     ulid.Make().String(),
					RideID: rideID,
					Status: "ARRIVED",
				})
			}
		}
	}
	fmt.Printf("chairPostCoordinateWorkerRunFunc: insertStatuses: %#v\n", insertStatuses)

	if len(insertStatuses) > 0 {
		_, err := tx.NamedExecContext(ctx, "INSERT INTO ride_statuses (id, ride_id, status) VALUES (:id), (:ride_id), (:status)", insertStatuses)
		if err != nil {
			slog.Error("chairPostCoordinate: failed to Insert statuses", "error", err)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		slog.Error("chairPostCoordinate: failed to Commit", "error", err)
		return
	}

}

type simpleUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type chairGetNotificationResponse struct {
	Data         *chairGetNotificationResponseData `json:"data"`
	RetryAfterMs int                               `json:"retry_after_ms"`
}

type chairGetNotificationResponseData struct {
	RideID                string     `json:"ride_id"`
	User                  simpleUser `json:"user"`
	PickupCoordinate      Coordinate `json:"pickup_coordinate"`
	DestinationCoordinate Coordinate `json:"destination_coordinate"`
	Status                string     `json:"status"`
}

func chairGetNotification(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	chair := ctx.Value("chair").(*Chair)

	tx, err := db.Beginx()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer tx.Rollback()
	ride := &Ride{}
	yetSentRideStatus := RideStatus{}
	status := ""

	if err := tx.GetContext(ctx, ride, `SELECT * FROM rides WHERE chair_id = ? ORDER BY updated_at DESC LIMIT 1`, chair.ID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusOK, &chairGetNotificationResponse{
				RetryAfterMs: 3000,
			})
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if err := tx.GetContext(ctx, &yetSentRideStatus, `SELECT * FROM ride_statuses WHERE ride_id = ? AND chair_sent_at IS NULL ORDER BY created_at ASC LIMIT 1`, ride.ID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			status, err = getLatestRideStatus(ctx, tx, ride.ID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
		} else {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	} else {
		status = yetSentRideStatus.Status
	}

	user := &User{}
	err = tx.GetContext(ctx, user, "SELECT * FROM users WHERE id = ? FOR SHARE", ride.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if yetSentRideStatus.ID != "" {
		_, err := tx.ExecContext(ctx, `UPDATE ride_statuses SET chair_sent_at = CURRENT_TIMESTAMP(6) WHERE id = ?`, yetSentRideStatus.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, &chairGetNotificationResponse{
		Data: &chairGetNotificationResponseData{
			RideID: ride.ID,
			User: simpleUser{
				ID:   user.ID,
				Name: fmt.Sprintf("%s %s", user.Firstname, user.Lastname),
			},
			PickupCoordinate: Coordinate{
				Latitude:  ride.PickupLatitude,
				Longitude: ride.PickupLongitude,
			},
			DestinationCoordinate: Coordinate{
				Latitude:  ride.DestinationLatitude,
				Longitude: ride.DestinationLongitude,
			},
			Status: status,
		},
		RetryAfterMs: 3000,
	})
}

type postChairRidesRideIDStatusRequest struct {
	Status string `json:"status"`
}

func chairPostRideStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rideID := r.PathValue("ride_id")

	chair := ctx.Value("chair").(*Chair)

	req := &postChairRidesRideIDStatusRequest{}
	if err := bindJSON(r, req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	tx, err := db.Beginx()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer tx.Rollback()

	ride := &Ride{}
	if err := tx.GetContext(ctx, ride, "SELECT * FROM rides WHERE id = ? FOR UPDATE", rideID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, errors.New("ride not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if ride.ChairID.String != chair.ID {
		writeError(w, http.StatusBadRequest, errors.New("not assigned to this ride"))
		return
	}

	switch req.Status {
	// Acknowledge the ride
	case "ENROUTE":
		if _, err := tx.ExecContext(ctx, "INSERT INTO ride_statuses (id, ride_id, status) VALUES (?, ?, ?)", ulid.Make().String(), ride.ID, "ENROUTE"); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	// After Picking up user
	case "CARRYING":
		status, err := getLatestRideStatus(ctx, tx, ride.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if status != "PICKUP" {
			writeError(w, http.StatusBadRequest, errors.New("chair has not arrived yet"))
			return
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO ride_statuses (id, ride_id, status) VALUES (?, ?, ?)", ulid.Make().String(), ride.ID, "CARRYING"); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	default:
		writeError(w, http.StatusBadRequest, errors.New("invalid status"))
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
