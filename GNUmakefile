default: testacc

# Run acceptance tests against the sandbox
.PHONY: testacc
testacc:
	TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m
