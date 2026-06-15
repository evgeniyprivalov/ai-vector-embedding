
.PHONY: migrations-local
migrations-local:
	set -a; . ./.env; set +a && goose -dir=./migrations -allow-missing up
