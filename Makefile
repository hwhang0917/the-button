.PHONY: all web bin

all: web bin

web:
	cd web && npm run build

bin:
	go build -o thebutton .
