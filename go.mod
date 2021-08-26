module go.f110.dev/mono

go 1.14

require (
	cloud.google.com/go/storage v1.6.0
	github.com/BurntSushi/xgb v0.0.0-20160522181843-27f122750802
	github.com/JuulLabs-OSS/cbgo v0.0.2
	github.com/aws/aws-sdk-go v1.35.20
	github.com/bradleyfalzon/ghinstallation v1.1.1
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/deckarep/golang-set v1.7.1 // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/fsnotify/fsnotify v1.4.9
	github.com/go-ble/ble v0.0.0-20210519192345-b055c211937b
	github.com/go-git/go-git/v5 v5.4.2
	github.com/go-sql-driver/mysql v1.5.0
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	github.com/golang/protobuf v1.4.3
	github.com/google/go-containerregistry v0.4.0
	github.com/google/go-github/v29 v29.0.3
	github.com/google/go-github/v32 v32.0.0
	github.com/google/uuid v1.1.2
	github.com/google/zoekt v0.0.0-20210819084712-fcc0c9ab67c5
	github.com/gopherjs/gopherjs v0.0.0-20181103185306-d547d1d9531e // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.1.0 // indirect
	github.com/hashicorp/consul/api v1.8.1
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/hashicorp/vault/api v1.0.4
	github.com/jarcoal/httpmock v1.0.7
	github.com/minio/minio v0.0.0-20210407225602-2899cc92b45f
	github.com/minio/minio-go/v6 v6.0.50
	github.com/minio/minio-go/v7 v7.0.11-0.20210302210017-6ae69c73ce78
	github.com/minio/minio-operator v0.0.0-20200214151316-3c7e5ae1c8a5
	github.com/mitchellh/mapstructure v1.3.2 // indirect
	github.com/peco/peco v0.5.8
	github.com/prometheus/client_golang v1.8.0
	github.com/shirou/gopsutil/v3 v3.21.1
	github.com/shurcooL/githubv4 v0.0.0-20210725200734-83ba7b4c9228
	github.com/shurcooL/graphql v0.0.0-20181231061246-d48a9a75455f // indirect
	github.com/sourcegraph/go-diff v0.5.1
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	go.etcd.io/etcd/v3 v3.3.0-rc.0.0.20200925060232-add86bbd1a7a
	go.f110.dev/protoc-ddl v0.0.0-20201210124226-127db5500265
	go.uber.org/zap v1.15.0
	golang.org/x/crypto v0.0.0-20210421170649-83a5a9bb288b
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/term v0.0.0-20210220032956-6a3ed077a48d
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1
	google.golang.org/api v0.22.0
	google.golang.org/genproto v0.0.0-20200806141610-86f49bd18e98 // indirect
	google.golang.org/grpc v1.29.1
	google.golang.org/protobuf v1.25.0
	gopkg.in/src-d/go-git.v4 v4.13.1
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776
	k8s.io/api v0.21.0
	k8s.io/apimachinery v0.21.0
	k8s.io/client-go v0.21.0
	k8s.io/component-base v0.19.6
	k8s.io/gengo v0.0.0-20201214224949-b6c5ce23f027
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.8.0
	sigs.k8s.io/kind v0.9.0
	sigs.k8s.io/yaml v1.2.0
	software.sslmate.com/src/go-pkcs12 v0.0.0-20200408181440-2981468c0ff3
)

replace github.com/dgrijalva/jwt-go => github.com/golang-jwt/jwt v3.2.1+incompatible
