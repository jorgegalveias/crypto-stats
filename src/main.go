/* Resources
-> Binance API:
	https://binance.github.io/binance-api-swagger/#/Market/get_api_v3_klines
-> CSV data endpoint
	https://data.binance.vision/?prefix=data/spot/monthly/klines/SHIBUSDT/1d/
->
*/

/*
Documentation
type Kline struct {
	OpenTime                 int64  //`json:"openTime"`
	Open                     string //`json:"open"`
	High                     string //`json:"high"`
	Low                      string //`json:"low"`
	Close                    string //`json:"close"`
	Volume                   string //`json:"volume"`
	CloseTime                int64  //`json:"closeTime"`
	QuoteAssetVolume         string //`json:"quoteAssetVolume"`
	TradeNum                 int64  //`json:"tradeNum"`
	TakerBuyBaseAssetVolume  string //`json:"takerBuyBaseAssetVolume"`
	TakerBuyQuoteAssetVolume string //`json:"takerBuyQuoteAssetVolume"`
	Unknown                  string
}
*/

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"strconv"

	"os"

	"github.com/montanaflynn/stats"
	"github.com/tomlazar/table"
	"gopkg.in/yaml.v3"
)

/*var (
	apiKey    = "your api key"
	secretKey = "your secret key"
)*/
const currencyPair = "SHIBUSDT"
const binanceURL = "https://api.binance.com/api/v3/klines?symbol=" + currencyPair + "&interval=1d&limit=1000"

// Kline define kline info
type CandleType string

const (
	BEAR CandleType = "BEAR"
	BULL CandleType = "BULL"
	DOJI CandleType = "DOJI"
)

func (candleType *CandleType) calculateCandleType(openPrice float64, closePrice float64) CandleType {
	switch result := closePrice - openPrice; {
	case result > 0:
		return BULL
	case result < 0:
		return BEAR
	case result == 0:
		return DOJI
	default:
		return DOJI
	}
}

type BaseStatsCalculator struct{}

func (statsCalculator *BaseStatsCalculator) calculateStd(values []float64) float64 {
	return 0.0
}

type StatsCalculator interface {
	calculateValue(candle *Candle) float64
}

type Stats struct {
	Name        string
	Std         float64
	Mean        float64
	ValueAtRisk float64 `yaml:"value-at-risk"`
}

type ReturnPercentageStats struct {
	Stats
}

func calculateReturnPercentageValue(candle *Candle) float64 {
	return (candle.Close - candle.Open) / candle.Open
}

type AnalysisReport struct {
	currencyPair    string           `yaml:"currency-pair"`
	candleStatsList []CandleStats    `yaml:"-"`
	stats           map[string]Stats `yaml:"stats"`
}

type CandleStats struct {
	Candle
	candleType       CandleType
	returnPercentage float64
}

type Candle struct {
	OpenTime  int64   //`json:"openTime"`
	Open      float64 //`json:"open"`
	High      float64 //`json:"high"`
	Low       float64 //`json:"low"`
	Close     float64 //`json:"close"`
	Volume    int64   //`json:"volume"`
	CloseTime int64   //`json:"closeTime"`
}

func main() {
	client := http.Client{}

	req, err := http.NewRequest("GET", binanceURL, nil)

	req.Header.Add("accept", "application/json")

	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.Do(req)
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)

	bodyString := string(bodyBytes)

	fmt.Println(bodyString)

	var candlesRaw [][]interface{}

	json.Unmarshal(bodyBytes, &candlesRaw)

	var candleList *[]Candle = new(Candle).convertFromArray(candlesRaw)

	fmt.Println(candleList)

	var candleStatsList []CandleStats = calculateCandleStats(candleList)
	var report AnalysisReport = calculateReport(candleStatsList)

	fmt.Println(report.stats)

	data, _ := yaml.Marshal(&report.stats)

	fmt.Println(string(data))

	err = ioutil.WriteFile("report.yaml", data, 0644)

	if err != nil {

		log.Fatal(err)
	}

	outputTable := buildTable(report.stats)
	err = outputTable.WriteTable(os.Stdout, nil)
}

func buildTable(stats map[string]Stats) *table.Table {

	var statsTable table.Table = table.Table{}

	statsTable.Headers = []string{"KPI Name", "Standard Deviation", "Mean", "Value At Risk"}
	statsTable.Rows = [][]string{}
	for _, v := range stats {
		row := []string{v.Name, fmt.Sprintf("%.2f", v.Std*100), fmt.Sprintf("%.2f", v.Mean*100), fmt.Sprintf("%.2f", v.ValueAtRisk*100)}
		statsTable.Rows = append(statsTable.Rows, row)
	}

	return &statsTable
}

func calculateReport(candleStatsList []CandleStats) AnalysisReport {

	var values []float64 = []float64{}

	for _, v := range candleStatsList {
		values = append(values, math.Abs(v.returnPercentage))
	}

	var returnPercentageStats Stats = Stats{}

	returnPercentageStats.Name = "Return Percentage"
	returnPercentageStats.Std, _ = stats.StandardDeviationPopulation(values)
	returnPercentageStats.Mean, _ = stats.Mean(values)
	returnPercentageStats.ValueAtRisk = 2.88 * returnPercentageStats.Std

	report := AnalysisReport{
		currencyPair:    currencyPair,
		candleStatsList: candleStatsList,
		stats:           map[string]Stats{"return_percentage": returnPercentageStats},
	}
	return report

}

func calculateCandleStats(candleList *[]Candle) []CandleStats {

	candleStatsList := []CandleStats{}

	for _, candle := range *candleList {

		var candleStats *CandleStats = &CandleStats{}

		candleStats.Candle = candle

		candleStats.candleType = new(CandleType).calculateCandleType(candle.Open, candle.Close)
		candleStats.returnPercentage = calculateReturnPercentageValue(&candle)

		candleStatsList = append(candleStatsList, *candleStats)
	}
	return candleStatsList
}

func (*Candle) convertFromArray(rawArray [][]interface{}) *[]Candle {

	var candleList []Candle = []Candle{}

	for _, v := range rawArray {

		var candle *Candle = &Candle{}

		candle.OpenTime = int64(v[0].(float64))
		candle.Open, _ = strconv.ParseFloat(v[1].(string), 64)
		candle.High, _ = strconv.ParseFloat(v[2].(string), 64)
		candle.Low, _ = strconv.ParseFloat(v[3].(string), 64)
		candle.Close, _ = strconv.ParseFloat(v[4].(string), 64)
		candle.Volume, _ = strconv.ParseInt(v[5].(string), 10, 64)
		candle.CloseTime = int64(v[6].(float64))

		candleList = append(candleList, *candle)
	}
	return &candleList
}
