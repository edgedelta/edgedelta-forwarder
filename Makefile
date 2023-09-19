GIT_PROJECT_ROOT := $(shell git rev-parse --show-toplevel)

release-forwarder: # trigger new lambda extension release (requires VERSION)
	@bash $(GIT_PROJECT_ROOT)/deploy/trigger_release.sh $(VERSION)

push-forwarder: # push lambda extension to dev aws account (requires ARCH and VERSION)
	@bash $(GIT_PROJECT_ROOT)/deploy/build.sh dev $(ARCH) $(VERSION)
	@bash $(GIT_PROJECT_ROOT)/deploy/publish.sh dev $(ARCH) $(VERSION)