TARGET = hdt
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
	GOOS=linux GOARCH=$(GOARCH) go build -o $(TARGET)

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: vendor
vendor: tidy
	go mod vendor

.PHONY: clean
clean:
	rm -f $(TARGET)
