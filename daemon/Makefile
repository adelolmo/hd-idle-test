TARGET = hdtd
BIN_DIR=/usr/bin
PLATFORM := $(shell uname -m)

ARCH :=
	ifeq ($(PLATFORM),x86_64)
		ARCH = amd64
	endif
	ifeq ($(PLATFORM),aarch64)
		ARCH = arm64
	endif
	ifeq ($(PLATFORM),armv7l)
		ARCH = armhf
	endif
GOARCH :=
	ifeq ($(ARCH),amd64)
		GOARCH = amd64
	endif
	ifeq ($(ARCH),i386)
		GOARCH = 386
	endif
	ifeq ($(ARCH),arm64)
		GOARCH = arm64
	endif
	ifeq ($(ARCH),armhf)
		GOARCH = arm
	endif

ifeq ($(GOARCH),)
  $(error Invalid ARCH: $(ARCH))
endif

$(TARGET):
	GOOS=linux GOARCH=$(GOARCH) GO111MODULE=on go build -o $(TARGET)

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: vendor
vendor: tidy
	go mod vendor

.PHONY: clean
clean:
	rm -f $(TARGET)

.PHONY: install
install:
	install -Dm755 $(TARGET) $(DESTDIR)$(BIN_DIR)/$(TARGET)
	install -Dm644 $(TARGET).service $(DESTDIR)/lib/systemd/system/$(TARGET).service
	systemctl daemon-reload

.PHONY: uninstall
uninstall:
	rm -f $(DESTDIR)$(BIN_DIR)/$(TARGET)
	rm -f $(DESTDIR)/lib/systemd/system/$(TARGET).service