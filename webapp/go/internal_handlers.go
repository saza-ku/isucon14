package main

import (
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

var (
	chairsForMatching1   = map[string]*ChairWithSpeed{}
	chairsForMatchingMu1 sync.RWMutex
	chairsForMatching2   = map[string]*ChairWithSpeed{}
	chairsForMatchingMu2 sync.RWMutex
)

func PushChairForMatching(coordinate Coordinate, chair *ChairWithSpeed) {
	fmt.Println("PushChairForMatching")
	if abs(coordinate.Latitude) <= 150 && abs(coordinate.Longitude) <= 150 {
		chairsForMatchingMu1.Lock()
		chairsForMatching1[chair.ID] = chair
		chairsForMatchingMu1.Unlock()
	} else {
		chairsForMatchingMu2.Lock()
		chairsForMatching2[chair.ID] = chair
		chairsForMatchingMu2.Unlock()
	}
}

func GetFastestChairForMatching(coordinate Coordinate) *ChairWithSpeed {
	if abs(coordinate.Latitude) <= 150 && abs(coordinate.Longitude) <= 150 {
		chairsForMatchingMu1.RLock()
		defer chairsForMatchingMu1.RUnlock()

		var fastest *ChairWithSpeed
		for _, chair := range chairsForMatching1 {
			if fastest == nil || chair.Speed > fastest.Speed {
				fastest = chair
			}
		}

		return fastest
	} else {
		chairsForMatchingMu2.RLock()
		defer chairsForMatchingMu2.RUnlock()

		var fastest *ChairWithSpeed
		for _, chair := range chairsForMatching2 {
			if fastest == nil || chair.Speed > fastest.Speed {
				fastest = chair
			}
		}

		return fastest
	}

}

func DeleteChairForMatching(chairID string) {
	chairsForMatchingMu1.Lock()
	defer chairsForMatchingMu1.Unlock()
	if _, ok := chairsForMatching1[chairID]; ok {
		delete(chairsForMatching1, chairID)
		return
	}

	chairsForMatchingMu2.Lock()
	defer chairsForMatchingMu2.Unlock()
	if _, ok := chairsForMatching2[chairID]; ok {
		delete(chairsForMatching2, chairID)
	}
}

// このAPIをインスタンス内から一定間隔で叩かせることで、椅子とライドをマッチングさせる
func internalGetMatching(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// MEMO: 一旦最も待たせているリクエストに適当な空いている椅子マッチさせる実装とする。おそらくもっといい方法があるはず…
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
	chair := GetFastestChairForMatching(coordinate)
	if chair != nil {
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
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	fmt.Println("fallback")

	matched := &Chair{}
	empty := false
	for i := 0; i < 10; i++ {
		if err := db.GetContext(ctx, matched, "SELECT * FROM chairs INNER JOIN (SELECT id FROM chairs WHERE is_active = TRUE ORDER BY RAND() LIMIT 1) AS tmp ON chairs.id = tmp.id LIMIT 1"); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			writeError(w, http.StatusInternalServerError, err)
		}

		if err := db.GetContext(ctx, &empty, "SELECT COUNT(*) = 0 FROM (SELECT COUNT(chair_sent_at) = 6 AS completed FROM ride_statuses WHERE ride_id IN (SELECT id FROM rides WHERE chair_id = ?) GROUP BY ride_id) is_completed WHERE completed = FALSE", matched.ID); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if empty {
			break
		}
	}
	if !empty {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if _, err := db.ExecContext(ctx, "UPDATE rides SET chair_id = ? WHERE id = ?", matched.ID, ride.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
