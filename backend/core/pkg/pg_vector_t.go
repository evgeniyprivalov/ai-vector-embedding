package pkg

import (
	"context"
	"database/sql"
	"embed"
	"testing"
	"time"

	"github.com/go-testfixtures/testfixtures/v3"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gitlab.com/evgeniyprivalov/golib/pg"
)

type PostgresqlTestEngine struct {
	t         *testing.T
	container *postgres.PostgresContainer
	pool      *pg.Pool
	dsn       string
}

// Terminate stops and removes the PostgreSQL container.
func (pte *PostgresqlTestEngine) Terminate() {
	pte.t.Helper()

	err := pte.container.Terminate(context.Background())
	require.NoError(pte.t, err)
}

// setMigrations applies database migrations using the provided migration options.
func (pte *PostgresqlTestEngine) setMigrations(mo migrationsOptionsInterface) error {
	db, err := sql.Open("postgres", pte.dsn)
	if err != nil {
		return err
	}
	defer db.Close() //nolint:errcheck

	goose.SetBaseFS(mo.GetMigrationsFS())

	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	return goose.Up(db, mo.GetMigrationsDir())
}

// SetFixtures loads test fixtures from the specified path into the database.
func (pte *PostgresqlTestEngine) SetFixtures(fixturesPath string) {
	pte.t.Helper()

	db, err := sql.Open("postgres", pte.pool.Config().ConnString())
	require.NoError(pte.t, err)
	defer db.Close() //nolint:errcheck

	fixtures, err := testfixtures.New(
		testfixtures.DangerousSkipTestDatabaseCheck(),
		testfixtures.Database(db),
		testfixtures.Dialect("postgres"),
		testfixtures.Directory(fixturesPath),
	)
	require.NoError(pte.t, err)

	err = fixtures.Load()
	require.NoError(pte.t, err)
}

// Pool returns the PostgreSQL connection pool.
func (pte *PostgresqlTestEngine) Pool() *pg.Pool {
	return pte.pool
}

// Truncate clears all tables in the database except for migration tables.
func (pte *PostgresqlTestEngine) Truncate() {
	pte.t.Helper()

	query := `
		BEGIN;
			DO $$ DECLARE
				tn text;
				sn text;
			BEGIN
				FOR tn IN (SELECT tablename FROM pg_tables WHERE schemaname=current_schema() AND tablename != 'goose_db_version') LOOP
					EXECUTE 'TRUNCATE TABLE public."' || tn || '" RESTART IDENTITY CASCADE;';
				END LOOP;


				FOR sn IN (SELECT sequence_name FROM information_schema.sequences WHERE sequence_schema=current_schema() AND sequence_name not like 'goose_db_version%') LOOP
					EXECUTE 'ALTER SEQUENCE ' || sn || ' RESTART;';
				END LOOP;
			END $$;
		COMMIT;
	`

	_, err := pte.pool.Exec(context.Background(), query)
	require.NoError(pte.t, err)
}

type migrationsOptionsInterface interface {
	GetMigrationsDir() string
	GetMigrationsFS() embed.FS
}

func PgVectorNew(t *testing.T, mo migrationsOptionsInterface) *PostgresqlTestEngine {
	t.Helper()

	ctx := context.Background()
	pgContainer, err := postgres.Run(
		ctx,
		"pgvector/pgvector:pg16",
		postgres.WithDatabase("unittest_db"),
		postgres.WithUsername("user"),
		postgres.WithPassword("password"),
		postgres.BasicWaitStrategies(),
		postgres.WithSQLDriver("pgx"),
		postgres.BasicWaitStrategies(),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second),
		),
	)
	require.NoError(t, err)

	dsn := pgContainer.MustConnectionString(ctx, "sslmode=disable")
	pool, err := pg.NewClient(ctx, pg.WithDSN(dsn))
	require.NoError(t, err)

	pte := &PostgresqlTestEngine{
		t:         t,
		container: pgContainer,
		pool:      pool,
		dsn:       dsn,
	}

	err = pte.setMigrations(mo)
	require.NoError(t, err)

	return pte
}
