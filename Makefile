battary-indicator:
	go build -o bin/battary-indicator cmd/battary-indicator/battary-indicator.go

yarurf-balance-checker:
    go get github.com/sirupsen/logrus gopkg.in/yaml.v3
	go build -o bin/yarurf-balance-checker cmd/yarurf-balance-checker/yarurf-balance-checker.go