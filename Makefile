run:
	@go install ./cmd/hollow-cli && hollow-cli

test:
	@gotest -v ./...

schema:
	@echo "Updating scehmas .."
