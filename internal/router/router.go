package router

import (
	"net/http"

	"agem/internal/handlers"
	"agem/internal/httpx"
	"agem/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func Router(st *store.Store) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	r.Get("/healthz", healthz)

	h := handlers.New(st)

	// v1 remains available for older clients and low-level collection-specific
	// operations. New QRing frontends should use the canonical v3 flow below.
	r.Route("/v1", func(r chi.Router) {
		r.Route("/users", func(r chi.Router) {
			r.Post("/", h.CreateUser)
			r.Get("/{id}", h.GetUser)
			r.Patch("/{id}", h.UpdateUser)
			r.Delete("/{id}", h.DeleteUser)
			r.Post("/{user_id}/devices", h.PairDeviceToUser)
			r.Get("/{user_id}/devices", h.ListUserDevices)
		})

		r.Route("/devices", func(r chi.Router) {
			r.Post("/", h.CreateDevice)
			r.Get("/{id}", h.GetDevice)
			r.Get("/by_uid/{device_uid}", h.GetDeviceByUID)
			r.Patch("/{id}", h.UpdateDevice)
			r.Delete("/{id}", h.DeleteDevice)

			r.Post("/{device_id}/steps15m", h.IngestSteps15mBulk)
			r.Get("/{device_id}/steps15m", h.ListSteps15mByDate)
			r.Get("/{device_id}/steps/daily", h.ListStepsDaily)
			r.Post("/{device_id}/activity/daily", h.UpsertDailyActivity)
			r.Get("/{device_id}/activity/daily", h.ListDailyActivity)

			r.Post("/{device_id}/health/metrics", h.IngestHealthMetricsBulk)
			r.Get("/{device_id}/health/metrics", h.ListHealthMetrics)
			r.Post("/{device_id}/events", h.IngestDeviceEventsBulk)
			r.Get("/{device_id}/events", h.ListDeviceEvents)

			r.Post("/{device_id}/sleep/summary", h.UpsertSleepSummary)
			r.Get("/{device_id}/sleep/summary", h.GetSleepSummaryByDate)
			r.Post("/{device_id}/sleep/segments", h.IngestSleepSegmentsBulk)
			r.Get("/{device_id}/sleep/segments", h.ListSleepSegmentsByDate)

			r.Post("/{device_id}/workouts", h.CreateWorkout)
			r.Get("/{device_id}/workouts", h.ListWorkouts)
			r.Patch("/{device_id}/workouts", h.UpdateWorkoutByStartUTC)
			r.Delete("/{device_id}/workouts", h.DeleteWorkoutByStartUTC)
		})

		r.Route("/user_devices", func(r chi.Router) {
			r.Patch("/{id}", h.UpdateUserDevice)
			r.Delete("/{id}", h.DeleteUserDevice)
		})
	})

	// Canonical QRing SDK API. v3 is intentionally a complete contract instead
	// of a thin wrapper over the older collection-oriented v1 endpoints.
	r.Route("/v3", func(r chi.Router) {
		r.Route("/wearable", func(r chi.Router) {
			r.Post("/sync", h.SyncWearable)
			r.Get("/range", h.GetWearableRange)
		})

		r.Route("/devices", func(r chi.Router) {
			r.Get("/by_uid/{device_uid}", h.GetWearableDeviceByUIDV3)
			r.Get("/{device_id}/wearable/range", h.GetWearableRangeByDeviceID)
		})

		r.Route("/users", func(r chi.Router) {
			r.Get("/{user_id}/wearable/range", h.GetUserWearableRange)
			r.Post("/{user_id}/devices", h.PairWearableDeviceV3)
			r.Get("/{user_id}/devices", h.ListWearableDevicesForUserV3)
			r.Patch("/{user_id}/devices/{device_uid}", h.UpdateWearablePairingV3)
			r.Delete("/{user_id}/devices/{device_uid}", h.UnpairWearableDeviceV3)
		})
	})

	return r
}

// healthz godoc
//
// @Summary Health check
// @Description Returns service health status.
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /healthz [get]
func healthz(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
