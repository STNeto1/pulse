package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/joho/godotenv/autoload"
)

// Service represents a service that interacts with a database.
type Service interface {
	// Health returns a map of health status information.
	// The keys and values in the map are service-specific.
	Health() map[string]string

	// Close terminates the database connection.
	// It returns an error if the connection cannot be closed.
	Close() error

	// Watch takes a channel to send updates
	// All tables/rows are monitored
	Watch(chan DBNotification)

	// SyncTables runs the script to enable Watch to listen to all changes
	// It returns an error if the query fails
	SyncTables() error
}

type service struct {
	db *pgxpool.Pool
}

var (
	database   = os.Getenv("DB_DATABASE")
	password   = os.Getenv("DB_PASSWORD")
	username   = os.Getenv("DB_USERNAME")
	port       = os.Getenv("DB_PORT")
	host       = os.Getenv("DB_HOST")
	schema     = os.Getenv("DB_SCHEMA")
	dbInstance *service
)

func New() Service {
	// Reuse Connection
	if dbInstance != nil {
		return dbInstance
	}
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&search_path=%s", username, password, host, port, database, schema)
	conn, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		log.Fatal(err)
	}

	dbInstance = &service{
		db: conn,
	}
	return dbInstance
}

// Health checks the health of the database connection by pinging the database.
// It returns a map with keys indicating various health statistics.
func (s *service) Health() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	stats := make(map[string]string)

	// Ping the database
	err := s.db.Ping(ctx)
	if err != nil {
		stats["status"] = "down"
		stats["error"] = fmt.Sprintf("db down: %v", err)
		log.Fatalf(fmt.Sprintf("db down: %v", err)) // Log the error and terminate the program
		return stats
	}

	// Database is up, add more statistics
	stats["status"] = "up"
	stats["message"] = "It's healthy"

	// Get database stats (like open connections, in use, idle, etc.)
	dbStats := s.db.Stat()
	stats["open_connections"] = fmt.Sprintf("%d", dbStats.TotalConns())
	stats["idle"] = strconv.Itoa(int(dbStats.IdleConns()))
	stats["acquired"] = strconv.Itoa(int(dbStats.AcquiredConns()))
	// stats["in_use"] = strconv.Itoa(dbStats.)
	stats["wait_count"] = strconv.FormatInt(dbStats.EmptyAcquireCount(), 10)
	stats["wait_duration"] = dbStats.AcquireDuration().String()
	stats["max_idle_closed"] = strconv.FormatInt(dbStats.MaxIdleDestroyCount(), 10)
	stats["max_lifetime_closed"] = strconv.FormatInt(dbStats.MaxLifetimeDestroyCount(), 10)

	// Evaluate stats to provide a health message
	if dbStats.TotalConns() > 40 { // Assuming 50 is the max for this example
		stats["message"] = "The database is experiencing heavy load."
	}

	if dbStats.EmptyAcquireCount() > 1000 {
		stats["message"] = "The database has a high number of wait events, indicating potential bottlenecks."
	}

	if dbStats.MaxIdleDestroyCount() > int64(dbStats.TotalConns())/2 {
		stats["message"] = "Many idle connections are being closed, consider revising the connection pool settings."
	}

	if dbStats.MaxLifetimeDestroyCount() > int64(dbStats.TotalConns())/2 {
		stats["message"] = "Many connections are being closed due to max lifetime, consider increasing max lifetime or revising the connection usage pattern."
	}

	return stats
}

// Close closes the database connection.
// It logs a message indicating the disconnection from the specific database.
// If the connection is successfully closed, it returns nil.
// If an error occurs while closing the connection, it returns the error.
func (s *service) Close() error {
	log.Printf("Disconnected from database: %s", database)
	s.db.Close()
	return nil
}

type DBNotification struct {
	Operation string      `json:"operation"`
	Table     string      `json:"table"`
	Data      interface{} `json:"data"`
}

// Watch listen for messages from the database
// It takes a DBNotification channel
// If it fails to acquire a connections, it kills the app
// If it fails to LISTEN to a channel, it kills the app
// If it fails to parse to wait for the notification or to parse the message, will ignore the error and continue
func (s *service) Watch(ch chan DBNotification) {
	conn, err := s.db.Acquire(context.Background())
	if err != nil {
		log.Fatalf("Unable to acquire connection: %v\n", err)
	}
	defer conn.Release()

	pgConn := conn.Conn()
	_, err = pgConn.Exec(context.Background(), "LISTEN pulse_watcher")
	if err != nil {
		log.Fatalf("Unable to start listening: %v\n", err)
	}

	for {
		select {
		default:
			rawNotification, err := pgConn.WaitForNotification(context.Background())
			if err != nil {
				log.Printf("Error waiting for notification: %v\n", err)
				time.Sleep(1 * time.Second) // Backoff on error
				continue
			}

			var dbNotification DBNotification
			if err := json.Unmarshal([]byte(rawNotification.Payload), &dbNotification); err != nil {
				log.Printf("Failed to parse Payload into DBNotification: %v | %v", err, rawNotification.Payload)
				time.Sleep(1 * time.Second) // Backoff on error
				continue
			}

			ch <- dbNotification
		}
	}

}

func (s *service) SyncTables() error {
	_, err := s.db.Exec(context.Background(), `CREATE OR REPLACE FUNCTION pulse_watcher() RETURNS trigger AS
$$
DECLARE
    payload JSON;
BEGIN

    IF (TG_OP = 'INSERT') THEN
        payload = json_build_object(
                'operation', lower(TG_OP),
                'table', TG_TABLE_NAME,
                'data', NEW);
        PERFORM pg_notify('pulse_watcher', payload::text);
    ELSIF (TG_OP = 'UPDATE') THEN
        FOR payload IN
            SELECT json_build_object(
                           'operation', lower(TG_OP),
                           'table', TG_TABLE_NAME,
                           'data', NEW)
            LOOP
                PERFORM pg_notify('pulse_watcher', payload::text);
            END LOOP;
    ELSIF (TG_OP = 'DELETE') THEN
        FOR payload IN
            SELECT json_build_object(
                           'operation', lower(TG_OP),
                           'table', TG_TABLE_NAME,
                           'data', OLD)
            LOOP
                PERFORM pg_notify('pulse_watcher', payload::text);
            END LOOP;
    END IF;

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
`)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(context.Background(), `DO
$$
    DECLARE
        rec RECORD;
    BEGIN
        FOR rec IN
            SELECT tablename
            FROM pg_tables
            WHERE schemaname = 'public'
            LOOP
                EXECUTE format('
            CREATE OR REPLACE TRIGGER %I_trigger
            AFTER INSERT OR UPDATE OR DELETE ON %I
            FOR EACH ROW EXECUTE FUNCTION pulse_watcher();
        ', rec.tablename, rec.tablename);
            END LOOP;
    END
$$;`)
	if err != nil {
		return err
	}

	return nil
}
