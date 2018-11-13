module github.com/jenkins-x/jx

require (
	cloud.google.com/go v0.25.0
	code.gitea.io/sdk v0.0.0-20180702024448-79a281c4e34a
	github.com/Azure/draft v0.15.0
	github.com/Azure/go-autorest v10.14.0+incompatible
	github.com/BurntSushi/toml v0.3.0
	github.com/IBM-Cloud/bluemix-go v0.0.0-20181008063305-d718d474c7c2
	github.com/Jeffail/gabs v1.1.0
	github.com/MakeNowJust/heredoc v0.0.0-20171113091838-e9091a26100e
	github.com/Masterminds/semver v1.4.2
	github.com/Netflix/go-expect v0.0.0-20180814212900-124a37274874
	github.com/Pallinder/go-randomdata v0.0.0-20180616180521-15df0648130a
	github.com/StackExchange/wmi v0.0.0-20180116203802-5d049714c4a6
	github.com/alecthomas/template v0.0.0-20160405071501-a0175ee3bccc // indirect
	github.com/alecthomas/units v0.0.0-20151022065526-2efee857e7cf // indirect
	github.com/alexflint/go-filemutex v0.0.0-20171028004239-d358565f3c3f
	github.com/andygrunwald/go-gerrit v0.0.0-20171029143327-95b11af228a1
	github.com/andygrunwald/go-jira v1.5.0
	github.com/aws/aws-sdk-go v1.15.50
	github.com/banzaicloud/bank-vaults v0.0.0-20181015112421-ca15a6960a3a
	github.com/beevik/etree v1.0.1
	github.com/beorn7/perks v0.0.0-20180321164747-3a771d992973
	github.com/blang/semver v3.5.1+incompatible
	github.com/bouk/monkey v1.0.0
	github.com/c2h5oh/datasize v0.0.0-20171227191756-4eba002a5eae
	github.com/cenkalti/backoff v2.0.0+incompatible
	github.com/chromedp/cdproto v0.0.0-20180720050708-57cf4773008d
	github.com/chromedp/chromedp v0.1.1
	github.com/codeship/codeship-go v0.0.0-20180717142545-7793ca823354
	github.com/cpuguy83/go-md2man v1.0.8
	github.com/danwakefield/fnmatch v0.0.0-20160403171240-cbb64ac3d964
	github.com/davecgh/go-spew v1.1.1
	github.com/denormal/go-gitignore v0.0.0-20180713143441-75ce8f3e513c
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/disintegration/imaging v1.4.2
	github.com/docker/spdystream v0.0.0-20170912183627-bc6354cbbc29
	github.com/emicklei/go-restful v2.8.0+incompatible // indirect
	github.com/emirpasic/gods v1.9.0
	github.com/fatih/color v1.7.0
	github.com/fatih/structs v1.0.0
	github.com/fsnotify/fsnotify v1.4.7
	github.com/gfleury/go-bitbucket-v1 v0.0.0-20180608194953-66450bf15655
	github.com/ghodss/yaml v1.0.0
	github.com/go-ini/ini v1.38.1
	github.com/go-ole/go-ole v1.2.1
	github.com/go-openapi/spec v0.17.1 // indirect
	github.com/gobwas/glob v0.2.3
	github.com/gogo/protobuf v1.1.1
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/protobuf v1.1.0
	github.com/golang/snappy v0.0.0-20180518054509-2e65f85255db // indirect
	github.com/google/btree v0.0.0-20180124185431-e89373fe6b4a
	github.com/google/go-github v17.0.0+incompatible
	github.com/google/go-querystring v0.0.0-20170111101155-53e6ce116135
	github.com/google/gofuzz v0.0.0-20170612174753-24818f796faf
	github.com/googleapis/gax-go v2.0.0+incompatible
	github.com/googleapis/gnostic v0.2.0
	github.com/gophercloud/gophercloud v0.0.0-20180721014243-9bb899a7c1d9
	github.com/gorilla/context v1.1.1
	github.com/gorilla/mux v1.6.2
	github.com/gorilla/securecookie v1.1.1
	github.com/gorilla/sessions v1.1.1
	github.com/gorilla/websocket v1.2.0
	github.com/gregjones/httpcache v0.0.0-20180305231024-9cad4c3443a7
	github.com/hashicorp/go-cleanhttp v0.5.0 // indirect
	github.com/hashicorp/go-multierror v1.0.0 // indirect
	github.com/hashicorp/go-retryablehttp v0.0.0-20180718195005-e651d75abec6 // indirect
	github.com/hashicorp/go-rootcerts v0.0.0-20160503143440-6bb64b370b90 // indirect
	github.com/hashicorp/go-sockaddr v0.0.0-20180320115054-6d291a969b86 // indirect
	github.com/hashicorp/go-version v0.0.0-20180716215031-270f2f71b1ee
	github.com/hashicorp/golang-lru v0.0.0-20180201235237-0fb14efe8c47
	github.com/hashicorp/hcl v0.0.0-20180404174102-ef8a98b0bbce
	github.com/hashicorp/vault v0.11.4
	github.com/heptio/sonobuoy v0.12.0
	github.com/hinshun/vt10x v0.0.0-20180809195222-d55458df857c
	github.com/hpcloud/tail v1.0.0
	github.com/imdario/mergo v0.3.5
	github.com/inconshreveable/mousetrap v1.0.0
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99
	github.com/jbrukh/bayesian v0.0.0-20161210175230-bf3f261f9a9c
	github.com/jenkins-x/chyle v0.0.0-20180226080600-68f7a93a63ec
	github.com/jenkins-x/draft-repo v0.0.0-20180417100212-2f66cc518135
	github.com/jenkins-x/golang-jenkins v0.0.0-20180919102630-65b83ad42314
	github.com/jmespath/go-jmespath v0.0.0-20160202185014-0b12d6b521d8
	github.com/json-iterator/go v0.0.0-20180701071628-ab8a2e0c74be
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51
	github.com/kevinburke/ssh_config v0.0.0-20180317175531-9fc7bb800b55
	github.com/knative/build v0.0.0-20180906201914-846036c8b91d
	github.com/knq/snaker v0.0.0-20180306023312-d9ad1e7f342a
	github.com/knq/sysutil v0.0.0-20180306023629-0218e141a794
	github.com/kr/pty v1.1.2
	github.com/magiconair/properties v1.8.0
	github.com/mailru/easyjson v0.0.0-20180823135443-60711f1a8329
	github.com/mattn/go-colorable v0.0.9
	github.com/mattn/go-isatty v0.0.3
	github.com/matttproud/golang_protobuf_extensions v1.0.1
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b
	github.com/mitchellh/go-homedir v0.0.0-20180523094522-3864e76763d9
	github.com/mitchellh/mapstructure v0.0.0-20180715050151-f15292f7a699
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd
	github.com/modern-go/reflect2 v0.0.0-20180701023420-4b7aa43c6742
	github.com/nlopes/slack v0.0.0-20180721202243-347a74b1ea30
	github.com/onsi/ginkgo v1.6.0
	github.com/onsi/gomega v1.4.1
	github.com/operator-framework/operator-sdk v0.0.0-20181011175812-913cbf711929
	github.com/pborman/uuid v0.0.0-20170612153648-e790cca94e6c
	github.com/pelletier/go-buffruneio v0.2.0
	github.com/pelletier/go-toml v1.2.0
	github.com/petar/GoLLRB v0.0.0-20130427215148-53be0d36a84c
	github.com/peterbourgon/diskv v2.0.1+incompatible
	github.com/petergtz/pegomock v0.0.0-20181008215750-9750219ad78b
	github.com/pierrec/lz4 v2.0.5+incompatible // indirect
	github.com/pkg/browser v0.0.0-20170505125900-c90ca0c84f15
	github.com/pkg/errors v0.8.0
	github.com/pmezard/go-difflib v1.0.0
	github.com/prometheus/client_golang v0.8.0
	github.com/prometheus/client_model v0.0.0-20180712105110-5c3871d89910
	github.com/prometheus/common v0.0.0-20180518154759-7600349dcfe1
	github.com/prometheus/procfs v0.0.0-20180705121852-ae68e2d4c00f
	github.com/rifflock/lfshook v0.0.0-20180227222202-bf539943797a
	github.com/rodaine/hclencoder v0.0.0-20180926060551-0680c4321930
	github.com/russross/blackfriday v1.5.1
	github.com/ryanuber/go-glob v0.0.0-20170128012129-256dc444b735 // indirect
	github.com/satori/go.uuid v1.2.0
	github.com/sergi/go-diff v1.0.0
	github.com/shirou/gopsutil v0.0.0-20180901134234-eb1f1ab16f2e
	github.com/shirou/w32 v0.0.0-20160930032740-bb4de0191aa4
	github.com/shurcooL/githubv4 v0.0.0-20180509030948-19298c78142b
	github.com/shurcooL/go v0.0.0-20180423040247-9e1955d9fb6e
	github.com/shurcooL/graphql v0.0.0-20180514000029-62c9ce094e75
	github.com/sirupsen/logrus v1.0.6
	github.com/spf13/afero v1.1.1
	github.com/spf13/cast v1.2.0
	github.com/spf13/cobra v0.0.3
	github.com/spf13/jwalterweatherman v0.0.0-20180109140146-7c0cea34c8ec
	github.com/spf13/pflag v1.0.1
	github.com/spf13/viper v1.0.2
	github.com/src-d/gcfg v1.3.0
	github.com/stoewer/go-strcase v1.0.1
	github.com/stretchr/testify v1.2.2
	github.com/trivago/tgo v1.0.1
	github.com/viniciuschiele/tarx v0.0.0-20151205142357-6e3da540444d
	github.com/wbrefvem/go-bitbucket v0.0.0-20180917214347-1c96061fe622
	github.com/xanzy/go-gitlab v0.0.0-20180814191223-f3bc634ab936
	github.com/xanzy/ssh-agent v0.2.0
	go.opencensus.io v0.14.0
	golang.org/x/crypto v0.0.0-20180723164146-c126467f60eb
	golang.org/x/image v0.0.0-20180708004352-c73c2afc3b81
	golang.org/x/net v0.0.0-20181005035420-146acd28ed58
	golang.org/x/oauth2 v0.0.0-20180620175406-ef147856a6dd
	golang.org/x/sync v0.0.0-20180314180146-1d60e4601c6f
	golang.org/x/sys v0.0.0-20180715085529-ac767d655b30
	golang.org/x/text v0.3.0
	golang.org/x/time v0.0.0-20180412165947-fbb02b2291d2
	golang.org/x/tools v0.0.0-20180723204246-ded554d0681e
	google.golang.org/api v0.0.0-20180724000608-2c45710c7f3f
	google.golang.org/appengine v1.1.0
	google.golang.org/genproto v0.0.0-20180722052100-02b4e9547331
	google.golang.org/grpc v1.13.0
	gopkg.in/AlecAivazis/survey.v1 v1.6.2
	gopkg.in/alecthomas/kingpin.v2 v2.2.6 // indirect
	gopkg.in/fsnotify.v1 v1.4.7 // indirect
	gopkg.in/inf.v0 v0.9.1
	gopkg.in/robfig/cron.v2 v2.0.0-20150107220207-be2e0b0deed5
	gopkg.in/src-d/go-billy.v4 v4.2.0
	gopkg.in/src-d/go-git.v4 v4.5.0
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7
	gopkg.in/warnings.v0 v0.1.2
	gopkg.in/yaml.v2 v2.2.1
	k8s.io/api v0.0.0-20180628040859-072894a440bd
	k8s.io/apiextensions-apiserver v0.0.0-20180621085152-bbc52469f98b
	k8s.io/apimachinery v0.0.0-20180621070125-103fd098999d
	k8s.io/client-go v8.0.0+incompatible
	k8s.io/code-generator v0.0.0-20181017053441-8c97d6ab64da
	k8s.io/gengo v0.0.0-20180718083919-906d99f89cd6
	k8s.io/helm v2.7.2+incompatible
	k8s.io/kube-openapi v0.0.0-20180719232738-d8ea2fe547a4
	k8s.io/metrics v0.0.0-20180620010437-b11cf31b380b
	k8s.io/test-infra v0.0.0-20181016234544-2c26f647f17a
)

replace k8s.io/test-infra v0.0.0-20181016234544-2c26f647f17a => github.com/jenkins-x/test-infra v0.0.0-20181017095642-0e6fed3d4d4d
