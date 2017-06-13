#!/usr/bin/make -f 

GO ?= $(shell which go)
ULOGD_SRC ?= /tmp
CGO_CFLAGS ?= "-I$(ULOGD_SRC)/include -I$(ULOGD_SRC)"

sources = main.go plugin/plugin.go plugin/plugin.c
objects = ${OBJDIR}/output_DROPSONDE.so

all: clean $(objects)

$(objects) : $(sources)
	$(GO) version
	$(GO) env
	CGO_CFLAGS=$(CGO_CFLAGS) $(GO) build -o $@ -buildmode=c-shared .

clean:
	rm -f $(objects)

