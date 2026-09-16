package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"agem/internal/handlers"
	"agem/internal/httpx"
	"agem/internal/store"
)

func Router(st *store.Store) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	// Health
	r.Get("/healthz", healthz)

	h := handlers.New(st)

	r.Route("/v1", func(r chi.Router) {
		// Users
		r.Route("/users", func(r chi.Router) {
			r.Post("/", h.CreateUser)
			r.Get("/{id}", h.GetUser)
			r.Patch("/{id}", h.UpdateUser)
			r.Delete("/{id}", h.DeleteUser)

			// Pair/list user devices
			r.Post("/{user_id}/devices", h.PairDeviceToUser)
			r.Get("/{user_id}/devices", h.ListUserDevices)
		})

		// Devices
		r.Route("/devices", func(r chi.Router) {
			r.Post("/", h.CreateDevice)
			r.Get("/{id}", h.GetDevice)
			r.Get("/by_uid/{device_uid}", h.GetDeviceByUID)
			r.Patch("/{id}", h.UpdateDevice)
			r.Delete("/{id}", h.DeleteDevice)

			// Steps
			r.Post("/{device_id}/steps15m", h.IngestSteps15mBulk)
			r.Get("/{device_id}/steps15m", h.ListSteps15mByDate)
			r.Get("/{device_id}/steps/daily", h.ListStepsDaily)
			r.Post("/{device_id}/activity/daily", h.UpsertDailyActivity)
			r.Get("/{device_id}/activity/daily", h.ListDailyActivity)

			// Generic qring sensor measurements and device notifications
			r.Post("/{device_id}/health/metrics", h.IngestHealthMetricsBulk)
			r.Get("/{device_id}/health/metrics", h.ListHealthMetrics)
			r.Post("/{device_id}/events", h.IngestDeviceEventsBulk)
			r.Get("/{device_id}/events", h.ListDeviceEvents)

			// Sleep
			r.Post("/{device_id}/sleep/summary", h.UpsertSleepSummary)
			r.Get("/{device_id}/sleep/summary", h.GetSleepSummaryByDate)
			r.Post("/{device_id}/sleep/segments", h.IngestSleepSegmentsBulk)
			r.Get("/{device_id}/sleep/segments", h.ListSleepSegmentsByDate)

			// Workouts
			r.Post("/{device_id}/workouts", h.CreateWorkout)
			r.Get("/{device_id}/workouts", h.ListWorkouts)
			r.Patch("/{device_id}/workouts", h.UpdateWorkoutByStartUTC)  // identify by start_utc in body
			r.Delete("/{device_id}/workouts", h.DeleteWorkoutByStartUTC) // identify by start_utc query
		})

		// Pairing record updates
		r.Route("/user_devices", func(r chi.Router) {
			r.Patch("/{id}", h.UpdateUserDevice)
			r.Delete("/{id}", h.DeleteUserDevice)
		})
	})

	r.Route("/v3", func(r chi.Router) {
		r.Route("/wearable", func(r chi.Router) {
			r.Post("/sync", h.SyncWearable)
			r.Get("/range", h.GetWearableRange)
		})
		r.Route("/users", func(r chi.Router) {
			r.Get("/{user_id}/wearable/range", h.GetUserWearableRange)
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
	httpx.JSON(w, 200, map[string]string{"status": "ok"})
}
