package reports

import (
	"io"
	"io/ioutil"
	"strings"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
)

type BlogBarReport struct {
	Name       string
	BlogWriter io.Writer
	JSFileName string
	JSLinkURI  string

	labels []string
	data   []string
}

func NewBlogBarReport(name string, blogWriter io.Writer, jsFileName string, jsLinkURI string) *BlogBarReport {
	if name == "" {
		name = "barChart"
	}
	if jsLinkURI == "" {
		jsLinkURI = jsFileName
	}
	return &BlogBarReport{Name: name, BlogWriter: blogWriter, JSFileName: jsFileName, JSLinkURI: jsLinkURI}
}

func (r *BlogBarReport) AddText(name string, value string) {
	r.labels = append(r.labels, `"`+name+`"`)
	r.data = append(r.data, value)
}

func (r *BlogBarReport) AddNumber(name string, value int) {
	ReportAddNumber(r, name, value)
}

func (r *BlogBarReport) Render() error {
	err := r.writeBlog()
	if err != nil {
		return err
	}
	js := r.createJs()
	err = ioutil.WriteFile(r.JSFileName, []byte(js), util.DefaultWritePermissions)
	if err != nil {
		return err
	}
	log.Logger().Infof("Generated JavaScript %s", util.ColorInfo(r.JSFileName))
	return nil
}

func (r *BlogBarReport) writeBlog() error {
	text := `
<canvas id="` + r.elementName() + `" width="400" height="200"></canvas>
<script type="text/javascript" src="` + r.JSLinkURI + `"></script>
</canvas>
`
	_, err := r.BlogWriter.Write([]byte(text))
	return err
}

func (r *BlogBarReport) createJs() string {
	label := strings.Title(r.Name)
	labelsJs := strings.Join(r.labels, ", ")
	dataJs := strings.Join(r.data, ", ")
	elementId := r.elementName()
	return `var ctx = document.getElementById("` + elementId + `").getContext('2d');
		var ` + elementId + ` = new Chart(ctx, {
		    type: 'bar',
		    data: {
		        labels: [` + labelsJs + `],
		        datasets: [{
		            label: '# of ` + label + `',
		            data: [` + dataJs + `],
		            backgroundColor: [
		                'rgba(255, 99, 132, 0.2)',
		                'rgba(54, 162, 235, 0.2)',
		                'rgba(255, 206, 86, 0.2)',
		                'rgba(75, 192, 192, 0.2)',
		                'rgba(153, 102, 255, 0.2)',
		                'rgba(255, 159, 64, 0.2)'
		            ],
		            borderColor: [
		                'rgba(255,99,132,1)',
		                'rgba(54, 162, 235, 1)',
		                'rgba(255, 206, 86, 1)',
		                'rgba(75, 192, 192, 1)',
		                'rgba(153, 102, 255, 1)',
		                'rgba(255, 159, 64, 1)'
		            ],
		            borderWidth: 1
		        }]
		    },
		    options: {
		        responsive: true,
		        /*
		        scales: {
		            yAxes: [{
		                ticks: {
		                    beginAtZero:true
		                }
		            }]
		        }
		        */
		    }
		});
		`
}

func (r *BlogBarReport) elementName() string {
	return r.Name + "Chart"
}
