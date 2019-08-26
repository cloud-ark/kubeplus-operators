module github.com/cloud-ark/kubeplus-operators/moodle

go 1.12

require (
	github.com/PuerkitoBio/purell v1.1.1
	github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/spdystream v0.0.0-20170912183627-bc6354cbbc29
	github.com/emicklei/go-restful v2.9.5+incompatible
	github.com/evanphx/json-patch v4.5.0+incompatible
	github.com/ghodss/yaml v1.0.0
	github.com/go-openapi/jsonpointer v0.19.2
	github.com/go-openapi/jsonreference v0.19.2
	github.com/go-openapi/spec v0.19.2
	github.com/go-openapi/swag v0.19.2
	github.com/gogo/protobuf v1.2.2-0.20190723190241-65acae22fc9d
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/groupcache v0.0.0-20180924190550-6f2cf27854a4
	github.com/golang/protobuf v1.3.1
	github.com/google/btree v1.0.0
	github.com/google/gofuzz v1.0.0
	github.com/googleapis/gnostic v0.2.0
	github.com/gregjones/httpcache v0.0.0-20180305231024-9cad4c3443a7
	github.com/hashicorp/golang-lru v0.5.0
	github.com/imdario/mergo v0.3.6
	github.com/json-iterator/go v1.1.7
	github.com/lib/pq v1.0.0
	github.com/mailru/easyjson v0.0.0-20190614124828-94de47d64c63
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd
	github.com/modern-go/reflect2 v1.0.1
	github.com/petar/GoLLRB v0.0.0-20130427215148-53be0d36a84c // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible
	github.com/spf13/pflag v1.0.3
	golang.org/x/crypto v0.0.0-20190611184440-5c40567a22f8
	golang.org/x/net v0.0.0-20190812203447-cdfb69ac37fc
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sys v0.0.0-20190616124812-15dcb6c0061f
	golang.org/x/text v0.3.2
	golang.org/x/time v0.0.0-20180412165947-fbb02b2291d2
	golang.org/x/tools v0.0.0-20190826060629-95c3470cfb70
	google.golang.org/appengine v1.6.1
	gopkg.in/inf.v0 v0.9.1
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0-20190826115001-5c0cbe2ae294
	k8s.io/apiextensions-apiserver v0.0.0-20190826122135-04b6c51fc03a
	k8s.io/apimachinery v0.0.0-20190826114657-e31a5531b558
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/code-generator v0.0.0-20190826114438-f795916aae3f
	k8s.io/gengo v0.0.0-20190822140433-26a664648505
	k8s.io/klog v0.4.0
	k8s.io/kube-openapi v0.0.0-20190709113604-33be087ad058
	sigs.k8s.io/yaml v1.1.0
)

replace k8s.io/client-go v11.0.0+incompatible => k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
