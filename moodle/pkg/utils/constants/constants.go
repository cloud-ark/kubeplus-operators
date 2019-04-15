package constants

const (
	API_VERSION    = "moodlecontroller.kubeplus/v1"
	MOODLE_KIND    = "Moodle"
	CONTAINER_NAME = "moodle"
	TIMEOUT        = 600 // 10mins

)

var PLUGIN_MAP = map[string]map[string]string{
	"profilecohort": {
		"downloadLink":  "https://moodle.org/plugins/download.php/17929/local_profilecohort_moodle35_2018092800.zip",
		"installFolder": "/var/www/html/local/",
	},
	"wiris": {
		"downloadLink":  "https://moodle.org/plugins/download.php/18916/filter_wiris_moodle36_2019020700.zip",
		"installFolder": "/var/www/html/filter/",
	},
}
