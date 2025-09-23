battary-indicator:
	go build -o bin/battary-indicator$(ext) cmd/battary-indicator/battary-indicator.go

yarurf-balance-checker:
	go get github.com/sirupsen/logrus gopkg.in/yaml.v3
	go build -o bin/yarurf-balance-checker$(ext) cmd/yarurf-balance-checker/yarurf-balance-checker.go

all: battary-indicator yarurf-balance-checker