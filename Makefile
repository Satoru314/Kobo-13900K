# Targets read the settings from .env.
include .env
export

# ai.json is at the repository root; make runs from there.
KOBO_AI_CONFIG ?= $(CURDIR)/ai.json
export KOBO_AI_CONFIG

.PHONY: migrate check
# Apply the database migrations. kobo does not apply them itself and refuses to
# start while any are pending.
migrate:
	go -C api run ./src/cmd/migrate

# Check the setup without starting kobo: commands, browser, directories and DB.
check:
	go -C api run ./src/cmd/kobo -check
