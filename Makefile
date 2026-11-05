# Targets read the settings from .env.
include .env
export

.PHONY: migrate
# Apply the database migrations. kobo does not apply them itself and refuses to
# start while any are pending.
migrate:
	go -C api run ./src/cmd/migrate
