package healthserver

import (
	"net/http"
	"os"
	"time"

	logger "gitlab.com/evgeniyprivalov/golib/observability/log"
	"gitlab.com/evgeniyprivalov/golib/pg"
	"gitlab.com/evgeniyprivalov/golib/redis"
)

func MakeHealthServer(
	addr,
	environment string,
	readTimeout time.Duration,
	logger *logger.Logger,
	pgConn *pg.Pool,
	rc *redis.Redis,
) http.Server {
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/health", healthCheck(logger, pgConn, rc))

	if environment == "local" {
		addr = "0.0.0.0:0"
	}

	return http.Server{
		Addr:        addr,
		Handler:     healthMux,
		ReadTimeout: readTimeout,
	}
}

func healthCheck(logger *logger.Logger, pg *pg.Pool, redis *redis.Redis) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if pg != nil {
			if err := pg.Ping(r.Context()); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"ok": false}`))

				logger.Error("pg is not ready", "error", err)

				os.Exit(1)
			}

			if _, err := pg.Exec(r.Context(), "SELECT current_timestamp;"); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"ok": false}`))

				logger.Error("pg is not ready", "error", err)

				os.Exit(1)
			}
		}

		if redis != nil {
			if err := redis.Ping(r.Context()).Err(); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"ok": false}`))

				logger.Error("redis is not ready", "error", err)

				os.Exit(1)
			}
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok": true}`))
	}
}
