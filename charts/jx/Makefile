CHART_REPO := http://jenkins-x-chartmuseum:8080
NAME := jx
OS := $(shell uname)

setup:
	helm repo add jenkins-x https://storage.googleapis.com/chartmuseum.jenkins-x.io

build: setup clean
	helm dependency build charts/jx
	helm lint charts/jx

install: clean setup build
	helm upgrade --install $(NAME) charts/jx

upgrade: clean setup build
	helm upgrade --install $(NAME) charts/jx

delete:
	helm delete --purge $(NAME)

clean: 
	rm -rf charts/jx/charts
	rm -rf charts/jx/${NAME}*.tgz
	rm -rf charts/jx/requirements.lock

release: setup clean build

ifeq ($(OS),Darwin)
	sed -i "" -e "s/version:.*/version: $(VERSION)/" charts/jx/Chart.yaml
	sed -i "" -e "s/tag:.*/tag: $(VERSION)/" charts/jx/values.yaml

else ifeq ($(OS),Linux)
	sed -i -e "s/version:.*/version: $(VERSION)/" charts/jx/Chart.yaml
	sed -i -e "s/tag:.*/tag: $(VERSION)/" charts/jx/values.yaml
endif

	helm package charts/jx
	curl --fail -u $(CHARTMUSEUM_USER):$(CHARTMUSEUM_PASS) --data-binary "@$(NAME)-$(VERSION).tgz" $(CHART_REPO)/api/charts
	rm -rf ${NAME}*.tgz
