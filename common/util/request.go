package util

import (
	"io/ioutil"
	"net/http"
	"strings"
)

func HttpRequest(url,method,str string) (bool,error) {
	//url := "http://10.211.55.2:8081"
	//method := "POST"

	//payload := strings.NewReader(`44c+wCdCyg62lOv1XCvxFQ==`)
	payload := strings.NewReader(str)

	client := &http.Client{
	}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		return "",err
	}
	req.Header.Add("Content-Type", "text/plain")

	res, err := client.Do(req)
	if err != nil {
		return "",err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "",err
	}
	//fmt.Println(string(body))
	return string(body),err
}
