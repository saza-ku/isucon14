package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sync"
)

type ChairWithSpeed struct {
	ID    string `db:"id"`
	Speed int    `db:"speed"`
}

func getFastestWithoutCache(coordinate Coordinate) *ChairWithSpeed {
	chairs, err := getEmptyChairs()
	if err != nil {
		fmt.Println("getEmptyChairs error: ", err)
		return nil
	}

	chairsWithSpeed := make([]*ChairWithSpeed, len(chairs))
	for i, chair := range chairs {
		model, ok := masterChairModels[chair.Model]
		if !ok {
			model = ChairModel{}
		}
		chairsWithSpeed[i] = &ChairWithSpeed{
			ID:    chair.ID,
			Speed: model.Speed,
		}
	}

	var fastest *ChairWithSpeed
	var fasterTime int
	fmt.Println("chairsWithSpeed: ", chairsWithSpeed)
	for _, chair := range chairsWithSpeed {
		chairLocation := &ChairLocation{}
		err = db.Get(
			chairLocation,
			`SELECT * FROM chair_locations WHERE chair_id = ? ORDER BY created_at DESC LIMIT 1`,
			chair.ID,
		)
		if err != nil {
			continue
		}

		chairCoordinate := Coordinate{
			Latitude:  chairLocation.Latitude,
			Longitude: chairLocation.Longitude,
		}
		distance := calculateDistance(coordinate.Latitude, coordinate.Longitude, chairCoordinate.Latitude, chairCoordinate.Longitude)
		time := distance / chair.Speed
		if fastest == nil || time < fasterTime {
			fastest = chair
			fasterTime = time
		}

		fmt.Println("distance: ", distance)
		fmt.Println("time: ", time)
		fmt.Println("speed: ", chair.Speed)
	}

	return fastest
}

func assignChairToRide() {
	ctx := context.Background()
	ride := &Ride{}
	if err := db.GetContext(ctx, ride, `SELECT * FROM rides WHERE chair_id IS NULL ORDER BY created_at LIMIT 1`); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return
		}
		return
	}

	coordinate := Coordinate{
		Latitude:  ride.PickupLatitude,
		Longitude: ride.PickupLongitude,
	}
	fmt.Println("matching: ", coordinate)
	chair := getFastestWithoutCache(coordinate)
	if chair != nil {
		fmt.Printf("matching chair: %v\n", chair)
		// TODO: ここでトランザクションを使う
		if _, err := db.ExecContext(ctx, "UPDATE rides SET chair_id = ? WHERE id = ?", chair.ID, ride.ID); err != nil {
			return
		}
		if _, err := db.ExecContext(ctx, "UPDATE chairs SET is_empty = FALSE WHERE id = ?", chair.ID); err != nil {
			return
		}
		return
	}
}

func getEmptyChairs() ([]Chair, error) {
	chairs := []Chair{}
	err := db.Select(&chairs, `
SELECT
    chairs.id,
	chairs.owner_id,
	chairs.name,
	chairs.model,
	chairs.is_active,
	chairs.is_empty,
	chairs.access_token,
	chairs.created_at,
	chairs.updated_at
FROM chairs
WHERE is_empty = TRUE AND is_active = TRUE
	`)
	if err != nil {
		return nil, err
	}

	return chairs, nil
}

// このAPIをインスタンス内から一定間隔で叩かせることで、椅子とライドをマッチングさせる
func internalGetMatching(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ride := &Ride{}
	if err := db.GetContext(ctx, ride, `SELECT * FROM rides WHERE chair_id IS NULL ORDER BY created_at LIMIT 1`); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	coordinate := Coordinate{
		Latitude:  ride.PickupLatitude,
		Longitude: ride.PickupLongitude,
	}
	fmt.Println("matching: ", coordinate)
	chair := getFastestWithoutCache(coordinate)
	if chair != nil {
		fmt.Printf("matching chair: %v\n", chair)
		if _, err := db.ExecContext(ctx, "UPDATE rides SET chair_id = ? WHERE id = ?", chair.ID, ride.ID); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
		/*
			var empty bool
			if err := db.GetContext(ctx, &empty, "SELECT COUNT(*) = 0 FROM (SELECT COUNT(chair_sent_at) = 6 AS completed FROM ride_statuses WHERE ride_id IN (SELECT id FROM rides WHERE chair_id = ?) GROUP BY ride_id) is_completed WHERE completed = FALSE", chair.ID); err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			if empty {
				if _, err := db.ExecContext(ctx, "UPDATE rides SET chair_id = ? WHERE id = ?", chair.ID, ride.ID); err != nil {
					writeError(w, http.StatusInternalServerError, err)
					return
				}
				fmt.Println("success")
				w.WriteHeader(http.StatusNoContent)
				return
			}
		*/
	}

	w.WriteHeader(http.StatusNoContent)
}

func getFastest(chairs []*ChairWithSpeed) *ChairWithSpeed {
	var fastest *ChairWithSpeed
	for _, chair := range chairs {
		// check if the chair is active
		var isActive bool
		if err := db.Get(&isActive, "SELECT is_active FROM chairs WHERE id = ?", chair.ID); err != nil {
			continue
		}
		if fastest == nil || chair.Speed > fastest.Speed {
			fastest = chair
		}
	}

	return fastest
}

var nearbyChairsCache = map[string][]*ChairWithSpeed{}
var nearbyChairsCacheMu = &sync.Mutex{}

func toKey(coordinate Coordinate) string {
	return fmt.Sprintf("%d,%d", coordinate.Latitude, coordinate.Longitude)
}

func getNearbyFastestChairs(coordinate Coordinate) *ChairWithSpeed {
	nearbyChairsCacheMu.Lock()
	defer nearbyChairsCacheMu.Unlock()
	if chairs, ok := nearbyChairsCache[toKey(coordinate)]; ok {
		fastest := getFastest(chairs)

		delete(nearbyChairsCache, toKey(coordinate))

		return fastest
	}

	fmt.Println("not found in cache")

	chairs, _, err := getNearbyChairs(context.Background(), db, coordinate, 70)
	if err != nil {
		return nil
	}
	chairsWithSpeed := make([]*ChairWithSpeed, len(chairs))
	for i, chair := range chairs {
		model, ok := masterChairModels[chair.Model]
		if !ok {
			model = ChairModel{}
		}
		chairsWithSpeed[i] = &ChairWithSpeed{
			ID:    chair.ID,
			Speed: model.Speed,
		}
	}

	fastest := getFastest(chairsWithSpeed)

	return fastest
}

func setNearbyChairs(coordinate Coordinate, chairs []*ChairWithSpeed) {
	nearbyChairsCacheMu.Lock()
	defer nearbyChairsCacheMu.Unlock()
	nearbyChairsCache[toKey(coordinate)] = chairs
}
