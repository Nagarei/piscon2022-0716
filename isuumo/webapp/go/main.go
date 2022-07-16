package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/labstack/gommon/log"
	"github.com/motoki317/sc"
)

const Limit = 20
const NazotteLimit = 50

var chairDb *sqlx.DB
var estateDb *sqlx.DB
var mySQLConnectionDataChair *MySQLConnectionEnv
var mySQLConnectionDataEstate *MySQLConnectionEnv
var chairSearchCondition ChairSearchCondition
var estateSearchCondition EstateSearchCondition

type InitializeResponse struct {
	Language string `json:"language"`
}

type Chair struct {
	ID          int64  `db:"id" json:"id"`
	Name        string `db:"name" json:"name"`
	Description string `db:"description" json:"description"`
	Thumbnail   string `db:"thumbnail" json:"thumbnail"`
	Price       int64  `db:"price" json:"price"`
	PriceRange  int64  `db:"price_range" json:"-"`
	Height      int64  `db:"height" json:"height"`
	HeightRange int64  `db:"height_range" json:"-"`
	Width       int64  `db:"width" json:"width"`
	WidthRange  int64  `db:"width_range" json:"-"`
	Depth       int64  `db:"depth" json:"depth"`
	DepthRange  int64  `db:"depth_range" json:"-"`
	Color       string `db:"color" json:"color"`
	Features    string `db:"features" json:"features"`
	Kind        string `db:"kind" json:"kind"`
	Popularity  int64  `db:"popularity" json:"-"`
	PopularityM int64  `db:"popularity_m" json:"-"`
	Stock       int64  `db:"stock" json:"-"`
	InStock     bool   `db:"in_stock" json:"-"`
}

type ChairSearchResponse struct {
	Count  int64   `json:"count"`
	Chairs []Chair `json:"chairs"`
}

type ChairListResponse struct {
	Chairs []Chair `json:"chairs"`
}

// Estate 物件
type Estate struct {
	ID          int64       `db:"id" json:"id"`
	Thumbnail   string      `db:"thumbnail" json:"thumbnail"`
	Name        string      `db:"name" json:"name"`
	Description string      `db:"description" json:"description"`
	Latitude    float64     `db:"latitude" json:"latitude"`
	Longitude   float64     `db:"longitude" json:"longitude"`
	Address     string      `db:"address" json:"address"`
	Rent        int64       `db:"rent" json:"rent"`
	RentRange   int64       `db:"rent_range" json:"-"`
	DoorHeight  int64       `db:"door_height" json:"doorHeight"`
	DoorHRange  int64       `db:"door_height_range" json:"-"`
	DoorWidth   int64       `db:"door_width" json:"doorWidth"`
	DoorWRange  int64       `db:"door_width_range" json:"-"`
	Features    string      `db:"features" json:"features"`
	Popularity  int64       `db:"popularity" json:"-"`
	PopularityM int64       `db:"popularity_m" json:"-"`
	Point       interface{} `db:"point" json:"-"`
}

// EstateSearchResponse estate/searchへのレスポンスの形式
type EstateSearchResponse struct {
	Count   int64    `json:"count"`
	Estates []Estate `json:"estates"`
}

type EstateListResponse struct {
	Estates []Estate `json:"estates"`
}

type Coordinate struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Coordinates struct {
	Coordinates []Coordinate `json:"coordinates"`
}

type Range struct {
	ID  int64 `json:"id"`
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}

type RangeCondition struct {
	Prefix string   `json:"prefix"`
	Suffix string   `json:"suffix"`
	Ranges []*Range `json:"ranges"`
}

type ListCondition struct {
	List []string `json:"list"`
}

type EstateSearchCondition struct {
	DoorWidth  RangeCondition `json:"doorWidth"`
	DoorHeight RangeCondition `json:"doorHeight"`
	Rent       RangeCondition `json:"rent"`
	Feature    ListCondition  `json:"feature"`
}

type ChairSearchCondition struct {
	Width   RangeCondition `json:"width"`
	Height  RangeCondition `json:"height"`
	Depth   RangeCondition `json:"depth"`
	Price   RangeCondition `json:"price"`
	Color   ListCondition  `json:"color"`
	Feature ListCondition  `json:"feature"`
	Kind    ListCondition  `json:"kind"`
}

type BoundingBox struct {
	// TopLeftCorner 緯度経度が共に最小値になるような点の情報を持っている
	TopLeftCorner Coordinate
	// BottomRightCorner 緯度経度が共に最大値になるような点の情報を持っている
	BottomRightCorner Coordinate
}

type MySQLConnectionEnv struct {
	Host     string
	Port     string
	User     string
	DBName   string
	Password string
}

type RecordMapper struct {
	Record []string

	offset int
	err    error
}

func (r *RecordMapper) next() (string, error) {
	if r.err != nil {
		return "", r.err
	}
	if r.offset >= len(r.Record) {
		r.err = fmt.Errorf("too many read")
		return "", r.err
	}
	s := r.Record[r.offset]
	r.offset++
	return s, nil
}

func (r *RecordMapper) NextInt() int {
	s, err := r.next()
	if err != nil {
		return 0
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		r.err = err
		return 0
	}
	return i
}

func (r *RecordMapper) NextFloat() float64 {
	s, err := r.next()
	if err != nil {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		r.err = err
		return 0
	}
	return f
}

func (r *RecordMapper) NextString() string {
	s, err := r.next()
	if err != nil {
		return ""
	}
	return s
}

func (r *RecordMapper) Err() error {
	return r.err
}

func NewMySQLConnectionEnv(hostEnvName string) *MySQLConnectionEnv {
	return &MySQLConnectionEnv{
		Host:     getEnv(hostEnvName, "127.0.0.1"),
		Port:     getEnv("MYSQL_PORT", "3306"),
		User:     getEnv("MYSQL_USER", "isucon"),
		DBName:   getEnv("MYSQL_DBNAME", "isuumo"),
		Password: getEnv("MYSQL_PASS", "isucon"),
	}
}

func getEnv(key, defaultValue string) string {
	val := os.Getenv(key)
	if val != "" {
		return val
	}
	return defaultValue
}

// ConnectDB isuumoデータベースに接続する
func (mc *MySQLConnectionEnv) ConnectDB() (*sqlx.DB, error) {
	dsn := fmt.Sprintf("%v:%v@tcp(%v:%v)/%v?interpolateParams=true", mc.User, mc.Password, mc.Host, mc.Port, mc.DBName)
	return sqlx.Open("mysql", dsn)
}

func init() {
	jsonText, err := ioutil.ReadFile("../fixture/chair_condition.json")
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
	json.Unmarshal(jsonText, &chairSearchCondition)

	jsonText, err = ioutil.ReadFile("../fixture/estate_condition.json")
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
	json.Unmarshal(jsonText, &estateSearchCondition)
}

var (
	chairDetailCache  *sc.Cache[int, *Chair]
	estateDetailCache *sc.Cache[int, *Estate]

	lowPricedChairCache  *sc.Cache[struct{}, []Chair]
	lowPricedEstateCache *sc.Cache[struct{}, []Estate]
)

func main() {

	go func() {
		log.Fatal(http.ListenAndServe(":6060", nil))
	}()

	// Echo instance
	e := echo.New()
	// e.Debug = true
	e.Logger.SetLevel(log.ERROR)

	// Middleware
	// e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Initialize
	e.POST("/initialize", initialize)

	// Chair Handler
	e.GET("/api/chair/:id", getChairDetail)
	e.POST("/api/chair", postChair)
	e.GET("/api/chair/search", searchChairs)
	e.GET("/api/chair/low_priced", getLowPricedChair)
	e.GET("/api/chair/search/condition", getChairSearchCondition)
	e.POST("/api/chair/buy/:id", buyChair)

	// Estate Handler
	e.GET("/api/estate/:id", getEstateDetail)
	e.POST("/api/estate", postEstate)
	e.GET("/api/estate/search", searchEstates)
	e.GET("/api/estate/low_priced", getLowPricedEstate)
	e.POST("/api/estate/req_doc/:id", postEstateRequestDocument)
	e.POST("/api/estate/nazotte", searchEstateNazotte)
	e.GET("/api/estate/search/condition", getEstateSearchCondition)
	e.GET("/api/recommended_estate/:id", searchRecommendedEstateWithChair)

	// mySQLConnectionData = NewMySQLConnectionEnv("")

	// var err error
	// db, err = mySQLConnectionData.ConnectDB()
	// if err != nil {
	// 	e.Logger.Fatalf("DB connection failed : %v", err)
	// }
	// db.SetMaxOpenConns(10)
	// defer db.Close()

	mySQLConnectionDataChair = NewMySQLConnectionEnv("MYSQL_CHAIR_HOST")

	var err error
	chairDb, err = mySQLConnectionDataChair.ConnectDB()
	if err != nil {
		e.Logger.Fatalf("DB connection failed : %v", err)
	}
	chairDb.SetMaxOpenConns(50)
	defer chairDb.Close()

	mySQLConnectionDataEstate = NewMySQLConnectionEnv("MYSQL_ESTATE_HOST")

	estateDb, err = mySQLConnectionDataEstate.ConnectDB()
	if err != nil {
		e.Logger.Fatalf("DB connection failed : %v", err)
	}
	estateDb.SetMaxOpenConns(50)
	defer estateDb.Close()

	chairDetailCache = sc.NewMust[int, *Chair](func(_ context.Context, id int) (*Chair, error) {
		var chair Chair
		query := `SELECT * FROM chair WHERE id = ?`
		err = chairDb.Get(&chair, query, id)
		return &chair, err
	}, 24*time.Hour, 24*time.Hour)
	estateDetailCache = sc.NewMust[int, *Estate](func(_ context.Context, id int) (*Estate, error) {
		var estate Estate
		query := `SELECT * FROM estate WHERE id = ?`
		err = estateDb.Get(&estate, query, id)
		return &estate, err
	}, 24*time.Hour, 24*time.Hour)
	lowPricedChairCache = sc.NewMust(func(_ context.Context, _ struct{}) ([]Chair, error) {
		var chairs []Chair
		query := `SELECT * FROM chair WHERE in_stock = 1 ORDER BY price ASC, id ASC LIMIT ?`
		err := chairDb.Select(&chairs, query, Limit)
		if err != nil {
			if err == sql.ErrNoRows {
				return []Chair{}, nil
			}
			return nil, err
		}
		return chairs, nil
	}, 24*time.Hour, 24*time.Hour)
	lowPricedEstateCache = sc.NewMust(func(_ context.Context, _ struct{}) ([]Estate, error) {
		estates := make([]Estate, 0, Limit)
		query := `SELECT * FROM estate ORDER BY rent ASC, id ASC LIMIT ?`
		err := estateDb.Select(&estates, query, Limit)
		if err != nil {
			if err == sql.ErrNoRows {
				return []Estate{}, nil
			}
			return nil, err
		}
		return estates, nil
	}, 24*time.Hour, 24*time.Hour)

	// Start server
	serverPort := fmt.Sprintf(":%v", getEnv("SERVER_PORT", "1323"))
	e.Logger.Fatal(e.Start(serverPort))
}

func initialize(c echo.Context) error {
	sqlDir := filepath.Join("..", "mysql", "db")
	paths := []string{
		filepath.Join(sqlDir, "0_Schema.sql"),
		// filepath.Join(sqlDir, "1_DummyEstateData.sql"),
		filepath.Join(sqlDir, "2_DummyChairData.sql"),
	}

	for _, p := range paths {
		sqlFile, _ := filepath.Abs(p)
		cmdStr := fmt.Sprintf("mysql -h %v -u %v -p%v -P %v %v < %v",
			mySQLConnectionDataChair.Host,
			mySQLConnectionDataChair.User,
			mySQLConnectionDataChair.Password,
			mySQLConnectionDataChair.Port,
			mySQLConnectionDataChair.DBName,
			sqlFile,
		)
		if err := exec.Command("bash", "-c", cmdStr).Run(); err != nil {
			c.Logger().Errorf("Initialize script error : %v", err)
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	paths = []string{
		filepath.Join(sqlDir, "0_Schema.sql"),
		filepath.Join(sqlDir, "1_DummyEstateData.sql"),
		// filepath.Join(sqlDir, "2_DummyChairData.sql"),
	}

	for _, p := range paths {
		sqlFile, _ := filepath.Abs(p)
		cmdStr := fmt.Sprintf("mysql -h %v -u %v -p%v -P %v %v < %v",
			mySQLConnectionDataEstate.Host,
			mySQLConnectionDataEstate.User,
			mySQLConnectionDataEstate.Password,
			mySQLConnectionDataEstate.Port,
			mySQLConnectionDataEstate.DBName,
			sqlFile,
		)
		if err := exec.Command("bash", "-c", cmdStr).Run(); err != nil {
			c.Logger().Errorf("Initialize script error : %v", err)
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	chairDetailCache.Purge()
	estateDetailCache.Purge()
	lowPricedChairCache.Purge()
	lowPricedEstateCache.Purge()

	return c.JSON(http.StatusOK, InitializeResponse{
		Language: "go",
	})
}

func getChairDetail(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.Echo().Logger.Errorf("Request parameter \"id\" parse error : %v", err)
		return c.NoContent(http.StatusBadRequest)
	}

	chair, err := chairDetailCache.Get(context.Background(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.Echo().Logger.Infof("requested id's chair not found : %v", id)
			return c.NoContent(http.StatusNotFound)
		}
		c.Echo().Logger.Errorf("Failed to get the chair from id : %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if chair.Stock <= 0 {
		c.Echo().Logger.Infof("requested id's chair is sold out : %v", id)
		return c.NoContent(http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, chair)
}

func postChair(c echo.Context) error {
	header, err := c.FormFile("chairs")
	if err != nil {
		c.Logger().Errorf("failed to get form file: %v", err)
		return c.NoContent(http.StatusBadRequest)
	}
	f, err := header.Open()
	if err != nil {
		c.Logger().Errorf("failed to open form file: %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	defer f.Close()
	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		c.Logger().Errorf("failed to read csv: %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	chairs := make([]Chair, 0, len(records))
	for _, row := range records {
		rm := RecordMapper{Record: row}
		id := rm.NextInt()
		name := rm.NextString()
		description := rm.NextString()
		thumbnail := rm.NextString()
		price := rm.NextInt()
		height := rm.NextInt()
		width := rm.NextInt()
		depth := rm.NextInt()
		color := rm.NextString()
		features := rm.NextString()
		kind := rm.NextString()
		popularity := rm.NextInt()
		stock := rm.NextInt()
		if err := rm.Err(); err != nil {
			c.Logger().Errorf("failed to read record: %v", err)
			return c.NoContent(http.StatusBadRequest)
		}
		chairs = append(chairs, Chair{
			ID:          int64(id),
			Name:        name,
			Description: description,
			Thumbnail:   thumbnail,
			Price:       int64(price),
			Height:      int64(height),
			Width:       int64(width),
			Depth:       int64(depth),
			Color:       color,
			Features:    features,
			Kind:        kind,
			Popularity:  int64(popularity),
			Stock:       int64(stock),
		})
	}
	if _, err := chairDb.NamedExec("INSERT INTO `chair` (id, name, description, thumbnail, price, height, width, depth, color, features, kind, popularity, stock) VALUES (:id, :name, :description, :thumbnail, :price, :height, :width, :depth, :color, :features, :kind, :popularity, :stock)", chairs); err != nil {
		c.Logger().Errorf("failed to insert chair: %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	lowPricedChairCache.Purge()

	return c.NoContent(http.StatusCreated)
}

func searchChairs(c echo.Context) error {
	conditions := make([]string, 0)
	params := make([]interface{}, 0)

	if c.QueryParam("priceRangeId") != "" {
		// chairPrice, err := getRange(chairSearchCondition.Price, c.QueryParam("priceRangeId"))
		// if err != nil {
		// 	c.Echo().Logger.Infof("priceRangeID invalid, %v : %v", c.QueryParam("priceRangeId"), err)
		// 	return c.NoContent(http.StatusBadRequest)
		// }

		// if chairPrice.Min != -1 {
		// 	conditions = append(conditions, "price >= ?")
		// 	params = append(params, chairPrice.Min)
		// }
		// if chairPrice.Max != -1 {
		// 	conditions = append(conditions, "price < ?")
		// 	params = append(params, chairPrice.Max)
		// }
		rangeIndex, err := strconv.Atoi(c.QueryParam("priceRangeId"))
		if err != nil && !(0 <= rangeIndex && rangeIndex <= 3) {
			return c.NoContent(http.StatusBadRequest)
		}
		conditions = append(conditions, "price_range="+strconv.Itoa(rangeIndex))
	}

	if c.QueryParam("heightRangeId") != "" {
		// chairHeight, err := getRange(chairSearchCondition.Height, c.QueryParam("heightRangeId"))
		// if err != nil {
		// 	c.Echo().Logger.Infof("heightRangeIf invalid, %v : %v", c.QueryParam("heightRangeId"), err)
		// 	return c.NoContent(http.StatusBadRequest)
		// }

		// if chairHeight.Min != -1 {
		// 	conditions = append(conditions, "height >= ?")
		// 	params = append(params, chairHeight.Min)
		// }
		// if chairHeight.Max != -1 {
		// 	conditions = append(conditions, "height < ?")
		// 	params = append(params, chairHeight.Max)
		// }
		rangeIndex, err := strconv.Atoi(c.QueryParam("heightRangeId"))
		if err != nil && !(0 <= rangeIndex && rangeIndex <= 3) {
			return c.NoContent(http.StatusBadRequest)
		}
		conditions = append(conditions, "height_range="+strconv.Itoa(rangeIndex))
	}

	if c.QueryParam("widthRangeId") != "" {
		// chairWidth, err := getRange(chairSearchCondition.Width, c.QueryParam("widthRangeId"))
		// if err != nil {
		// 	c.Echo().Logger.Infof("widthRangeID invalid, %v : %v", c.QueryParam("widthRangeId"), err)
		// 	return c.NoContent(http.StatusBadRequest)
		// }

		// if chairWidth.Min != -1 {
		// 	conditions = append(conditions, "width >= ?")
		// 	params = append(params, chairWidth.Min)
		// }
		// if chairWidth.Max != -1 {
		// 	conditions = append(conditions, "width < ?")
		// 	params = append(params, chairWidth.Max)
		// }
		rangeIndex, err := strconv.Atoi(c.QueryParam("widthRangeId"))
		if err != nil && !(0 <= rangeIndex && rangeIndex <= 3) {
			return c.NoContent(http.StatusBadRequest)
		}
		conditions = append(conditions, "width_range="+strconv.Itoa(rangeIndex))
	}

	if c.QueryParam("depthRangeId") != "" {
		// chairDepth, err := getRange(chairSearchCondition.Depth, c.QueryParam("depthRangeId"))
		// if err != nil {
		// 	c.Echo().Logger.Infof("depthRangeId invalid, %v : %v", c.QueryParam("depthRangeId"), err)
		// 	return c.NoContent(http.StatusBadRequest)
		// }

		// if chairDepth.Min != -1 {
		// 	conditions = append(conditions, "depth >= ?")
		// 	params = append(params, chairDepth.Min)
		// }
		// if chairDepth.Max != -1 {
		// 	conditions = append(conditions, "depth < ?")
		// 	params = append(params, chairDepth.Max)
		// }
		rangeIndex, err := strconv.Atoi(c.QueryParam("depthRangeId"))
		if err != nil && !(0 <= rangeIndex && rangeIndex <= 3) {
			return c.NoContent(http.StatusBadRequest)
		}
		conditions = append(conditions, "depth_range="+strconv.Itoa(rangeIndex))
	}

	if c.QueryParam("kind") != "" {
		conditions = append(conditions, "kind = ?")
		params = append(params, c.QueryParam("kind"))
	}

	if c.QueryParam("color") != "" {
		conditions = append(conditions, "color = ?")
		params = append(params, c.QueryParam("color"))
	}

	if c.QueryParam("features") != "" {
		for _, f := range strings.Split(c.QueryParam("features"), ",") {
			conditions = append(conditions, "features LIKE CONCAT('%', ?, '%')")
			params = append(params, f)
		}
	}

	if len(conditions) == 0 {
		c.Echo().Logger.Infof("Search condition not found")
		return c.NoContent(http.StatusBadRequest)
	}

	conditions = append(conditions, "`in_stock` = 1")

	page, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil {
		c.Logger().Infof("Invalid format page parameter : %v", err)
		return c.NoContent(http.StatusBadRequest)
	}

	perPage, err := strconv.Atoi(c.QueryParam("perPage"))
	if err != nil {
		c.Logger().Infof("Invalid format perPage parameter : %v", err)
		return c.NoContent(http.StatusBadRequest)
	}

	searchQuery := "SELECT * FROM chair WHERE "
	countQuery := "SELECT COUNT(*) FROM chair WHERE "
	searchCondition := strings.Join(conditions, " AND ")
	limitOffset := " ORDER BY popularity_m ASC, id ASC LIMIT ? OFFSET ?"

	var res ChairSearchResponse
	err = chairDb.Get(&res.Count, countQuery+searchCondition, params...)
	if err != nil {
		c.Logger().Errorf("searchChairs DB execution error : %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	chairs := []Chair{}
	params = append(params, perPage, page*perPage)
	err = chairDb.Select(&chairs, searchQuery+searchCondition+limitOffset, params...)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusOK, ChairSearchResponse{Count: 0, Chairs: []Chair{}})
		}
		c.Logger().Errorf("searchChairs DB execution error : %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	res.Chairs = chairs

	return c.JSON(http.StatusOK, res)
}

func buyChair(c echo.Context) error {
	m := echo.Map{}
	if err := c.Bind(&m); err != nil {
		c.Echo().Logger.Infof("post buy chair failed : %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	_, ok := m["email"].(string)
	if !ok {
		c.Echo().Logger.Info("post buy chair failed : email not found in request body")
		return c.NoContent(http.StatusBadRequest)
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.Echo().Logger.Infof("post buy chair failed : %v", err)
		return c.NoContent(http.StatusBadRequest)
	}

	tx, err := chairDb.Beginx()
	if err != nil {
		c.Echo().Logger.Errorf("failed to create transaction : %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	defer tx.Rollback()

	var chair Chair
	err = tx.QueryRowx("SELECT * FROM chair WHERE id = ? AND `in_stock` = 1 FOR UPDATE", id).StructScan(&chair)
	if err != nil {
		if err == sql.ErrNoRows {
			c.Echo().Logger.Infof("buyChair chair id \"%v\" not found", id)
			return c.NoContent(http.StatusNotFound)
		}
		c.Echo().Logger.Errorf("DB Execution Error: on getting a chair by id : %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	_, err = tx.Exec("UPDATE chair SET stock = stock - 1 WHERE id = ?", id)
	if err != nil {
		c.Echo().Logger.Errorf("chair stock update failed : %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	err = tx.Commit()
	if err != nil {
		c.Echo().Logger.Errorf("transaction commit error : %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	chairDetailCache.Forget(id)
	lowPricedChairCache.Purge()

	return c.NoContent(http.StatusOK)
}

func getChairSearchCondition(c echo.Context) error {
	return c.JSON(http.StatusOK, chairSearchCondition)
}

func getLowPricedChair(c echo.Context) error {
	chairs, err := lowPricedChairCache.Get(context.Background(), struct{}{})
	if err != nil {
		if err == sql.ErrNoRows {
			c.Logger().Error("getLowPricedChair not found")
			return c.JSON(http.StatusOK, ChairListResponse{[]Chair{}})
		}
		c.Logger().Errorf("getLowPricedChair DB execution error : %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, ChairListResponse{Chairs: chairs})
}

func getEstateDetail(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.Echo().Logger.Infof("Request parameter \"id\" parse error : %v", err)
		return c.NoContent(http.StatusBadRequest)
	}

	estate, err := estateDetailCache.Get(context.Background(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.Echo().Logger.Infof("getEstateDetail estate id %v not found", id)
			return c.NoContent(http.StatusNotFound)
		}
		c.Echo().Logger.Errorf("Database Execution error : %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, estate)
}

func getRange(cond RangeCondition, rangeID string) (*Range, error) {
	RangeIndex, err := strconv.Atoi(rangeID)
	if err != nil {
		return nil, err
	}

	if RangeIndex < 0 || len(cond.Ranges) <= RangeIndex {
		return nil, fmt.Errorf("Unexpected Range ID")
	}

	return cond.Ranges[RangeIndex], nil
}

func postEstate(c echo.Context) error {
	header, err := c.FormFile("estates")
	if err != nil {
		c.Logger().Errorf("failed to get form file: %v", err)
		return c.NoContent(http.StatusBadRequest)
	}
	f, err := header.Open()
	if err != nil {
		c.Logger().Errorf("failed to open form file: %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	defer f.Close()
	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		c.Logger().Errorf("failed to read csv: %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	estates := make([]Estate, 0, len(records))
	for _, row := range records {
		rm := RecordMapper{Record: row}
		id := rm.NextInt()
		name := rm.NextString()
		description := rm.NextString()
		thumbnail := rm.NextString()
		address := rm.NextString()
		latitude := rm.NextFloat()
		longitude := rm.NextFloat()
		rent := rm.NextInt()
		doorHeight := rm.NextInt()
		doorWidth := rm.NextInt()
		features := rm.NextString()
		popularity := rm.NextInt()
		if err := rm.Err(); err != nil {
			c.Logger().Errorf("failed to read record: %v", err)
			return c.NoContent(http.StatusBadRequest)
		}
		estates = append(estates, Estate{
			ID:          int64(id),
			Thumbnail:   thumbnail,
			Name:        name,
			Description: description,
			Latitude:    latitude,
			Longitude:   longitude,
			Address:     address,
			Rent:        int64(rent),
			DoorHeight:  int64(doorHeight),
			DoorWidth:   int64(doorWidth),
			Features:    features,
			Popularity:  int64(popularity),
		})
	}
	if _, err := estateDb.NamedExec("INSERT INTO `estate` (id, name, description, thumbnail, address, latitude, longitude, rent, door_height, door_width, features, popularity) VALUES (:id, :name, :description, :thumbnail, :address, :latitude, :longitude, :rent, :door_height, :door_width, :features, :popularity)", estates); err != nil {
		c.Logger().Errorf("failed to insert estate: %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	lowPricedEstateCache.Purge()

	return c.NoContent(http.StatusCreated)
}

func searchEstates(c echo.Context) error {
	conditions := make([]string, 0)
	params := make([]interface{}, 0)

	if c.QueryParam("doorHeightRangeId") != "" {
		// doorHeight, err := getRange(estateSearchCondition.DoorHeight, c.QueryParam("doorHeightRangeId"))
		// if err != nil {
		// 	c.Echo().Logger.Infof("doorHeightRangeID invalid, %v : %v", c.QueryParam("doorHeightRangeId"), err)
		// 	return c.NoContent(http.StatusBadRequest)
		// }

		// if doorHeight.Min != -1 {
		// 	conditions = append(conditions, "door_height >= ?")
		// 	params = append(params, doorHeight.Min)
		// }
		// if doorHeight.Max != -1 {
		// 	conditions = append(conditions, "door_height < ?")
		// 	params = append(params, doorHeight.Max)
		// }
		rangeIndex, err := strconv.Atoi(c.QueryParam("doorHeightRangeId"))
		if err != nil && !(0 <= rangeIndex && rangeIndex <= 3) {
			return c.NoContent(http.StatusBadRequest)
		}
		conditions = append(conditions, "door_height_range="+strconv.Itoa(rangeIndex))
	}

	if c.QueryParam("doorWidthRangeId") != "" {
		// doorWidth, err := getRange(estateSearchCondition.DoorWidth, c.QueryParam("doorWidthRangeId"))
		// if err != nil {
		// 	c.Echo().Logger.Infof("doorWidthRangeID invalid, %v : %v", c.QueryParam("doorWidthRangeId"), err)
		// 	return c.NoContent(http.StatusBadRequest)
		// }

		// if doorWidth.Min != -1 {
		// 	conditions = append(conditions, "door_width >= ?")
		// 	params = append(params, doorWidth.Min)
		// }
		// if doorWidth.Max != -1 {
		// 	conditions = append(conditions, "door_width < ?")
		// 	params = append(params, doorWidth.Max)
		// }
		rangeIndex, err := strconv.Atoi(c.QueryParam("doorWidthRangeId"))
		if err != nil && !(0 <= rangeIndex && rangeIndex <= 3) {
			return c.NoContent(http.StatusBadRequest)
		}
		conditions = append(conditions, "door_width_range="+strconv.Itoa(rangeIndex))
	}

	if c.QueryParam("rentRangeId") != "" {
		// estateRent, err := getRange(estateSearchCondition.Rent, c.QueryParam("rentRangeId"))
		// if err != nil {
		// 	c.Echo().Logger.Infof("rentRangeID invalid, %v : %v", c.QueryParam("rentRangeId"), err)
		// 	return c.NoContent(http.StatusBadRequest)
		// }
		//
		// if estateRent.Min != -1 {
		// 	conditions = append(conditions, "rent >= ?")
		// 	params = append(params, estateRent.Min)
		// }
		// if estateRent.Max != -1 {
		// 	conditions = append(conditions, "rent < ?")
		// 	params = append(params, estateRent.Max)
		// }

		rangeIndex, err := strconv.Atoi(c.QueryParam("rentRangeId"))
		if err != nil && !(0 <= rangeIndex && rangeIndex <= 3) {
			return c.NoContent(http.StatusBadRequest)
		}
		conditions = append(conditions, "rent_range="+strconv.Itoa(rangeIndex))
	}

	if c.QueryParam("features") != "" {
		for _, f := range strings.Split(c.QueryParam("features"), ",") {
			conditions = append(conditions, "features like concat('%', ?, '%')")
			params = append(params, f)
		}
	}

	if len(conditions) == 0 {
		c.Echo().Logger.Infof("searchEstates search condition not found")
		return c.NoContent(http.StatusBadRequest)
	}

	page, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil {
		c.Logger().Infof("Invalid format page parameter : %v", err)
		return c.NoContent(http.StatusBadRequest)
	}

	perPage, err := strconv.Atoi(c.QueryParam("perPage"))
	if err != nil {
		c.Logger().Infof("Invalid format perPage parameter : %v", err)
		return c.NoContent(http.StatusBadRequest)
	}

	searchQuery := "SELECT * FROM estate WHERE "
	countQuery := "SELECT COUNT(*) FROM estate WHERE "
	searchCondition := strings.Join(conditions, " AND ")
	limitOffset := " ORDER BY popularity_m ASC, id ASC LIMIT ? OFFSET ?"

	var res EstateSearchResponse
	err = estateDb.Get(&res.Count, countQuery+searchCondition, params...)
	if err != nil {
		c.Logger().Errorf("searchEstates DB execution error : %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	estates := []Estate{}
	params = append(params, perPage, page*perPage)
	err = estateDb.Select(&estates, searchQuery+searchCondition+limitOffset, params...)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusOK, EstateSearchResponse{Count: 0, Estates: []Estate{}})
		}
		c.Logger().Errorf("searchEstates DB execution error : %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	res.Estates = estates

	return c.JSON(http.StatusOK, res)
}

func getLowPricedEstate(c echo.Context) error {
	estates, err := lowPricedEstateCache.Get(context.Background(), struct{}{})
	if err != nil {
		if err == sql.ErrNoRows {
			c.Logger().Error("getLowPricedEstate not found")
			return c.JSON(http.StatusOK, EstateListResponse{[]Estate{}})
		}
		c.Logger().Errorf("getLowPricedEstate DB execution error : %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, EstateListResponse{Estates: estates})
}

func searchRecommendedEstateWithChair(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.Logger().Infof("Invalid format searchRecommendedEstateWithChair id : %v", err)
		return c.NoContent(http.StatusBadRequest)
	}

	chair := Chair{}
	query := `SELECT * FROM chair WHERE id = ?`
	err = chairDb.Get(&chair, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.Logger().Infof("Requested chair id \"%v\" not found", id)
			return c.NoContent(http.StatusBadRequest)
		}
		c.Logger().Errorf("Database execution error : %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	var estates []Estate
	w := chair.Width
	h := chair.Height
	d := chair.Depth
	query = `SELECT * FROM estate WHERE (door_width >= ? AND door_height >= ?) OR (door_width >= ? AND door_height >= ?) OR (door_width >= ? AND door_height >= ?) OR (door_width >= ? AND door_height >= ?) OR (door_width >= ? AND door_height >= ?) OR (door_width >= ? AND door_height >= ?) ORDER BY popularity_m ASC, id ASC LIMIT ?`
	err = estateDb.Select(&estates, query, w, h, w, d, h, w, h, d, d, w, d, h, Limit)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusOK, EstateListResponse{[]Estate{}})
		}
		c.Logger().Errorf("Database execution error : %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, EstateListResponse{Estates: estates})
}

func searchEstateNazotte(c echo.Context) error {
	coordinates := Coordinates{}
	err := c.Bind(&coordinates)
	if err != nil {
		c.Echo().Logger.Infof("post search estate nazotte failed : %v", err)
		return c.NoContent(http.StatusBadRequest)
	}

	if len(coordinates.Coordinates) == 0 {
		return c.NoContent(http.StatusBadRequest)
	}

	estates := []Estate{}
	query := fmt.Sprintf(`SELECT * FROM estate WHERE ST_Contains(ST_PolygonFromText(%s), point) ORDER BY popularity_m ASC, id ASC LIMIT ?`, coordinates.coordinatesToText())
	err = estateDb.Select(&estates, query, NazotteLimit)
	if err == sql.ErrNoRows {
		c.Echo().Logger.Infof("select * from estate where latitude ...", err)
		return c.JSON(http.StatusOK, EstateSearchResponse{Count: 0, Estates: []Estate{}})
	} else if err != nil {
		c.Echo().Logger.Errorf("database execution error : %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	var re EstateSearchResponse
	re.Estates = estates
	re.Count = int64(len(re.Estates))

	return c.JSON(http.StatusOK, re)
}

func postEstateRequestDocument(c echo.Context) error {
	m := echo.Map{}
	if err := c.Bind(&m); err != nil {
		c.Echo().Logger.Infof("post request document failed : %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	_, ok := m["email"].(string)
	if !ok {
		c.Echo().Logger.Info("post request document failed : email not found in request body")
		return c.NoContent(http.StatusBadRequest)
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.Echo().Logger.Infof("post request document failed : %v", err)
		return c.NoContent(http.StatusBadRequest)
	}

	_, err = estateDetailCache.Get(context.Background(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.NoContent(http.StatusNotFound)
		}
		c.Logger().Errorf("postEstateRequestDocument DB execution error : %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
}

func getEstateSearchCondition(c echo.Context) error {
	return c.JSON(http.StatusOK, estateSearchCondition)
}

func (cs Coordinates) getBoundingBox() BoundingBox {
	coordinates := cs.Coordinates
	boundingBox := BoundingBox{
		TopLeftCorner: Coordinate{
			Latitude: coordinates[0].Latitude, Longitude: coordinates[0].Longitude,
		},
		BottomRightCorner: Coordinate{
			Latitude: coordinates[0].Latitude, Longitude: coordinates[0].Longitude,
		},
	}
	for _, coordinate := range coordinates {
		if boundingBox.TopLeftCorner.Latitude > coordinate.Latitude {
			boundingBox.TopLeftCorner.Latitude = coordinate.Latitude
		}
		if boundingBox.TopLeftCorner.Longitude > coordinate.Longitude {
			boundingBox.TopLeftCorner.Longitude = coordinate.Longitude
		}

		if boundingBox.BottomRightCorner.Latitude < coordinate.Latitude {
			boundingBox.BottomRightCorner.Latitude = coordinate.Latitude
		}
		if boundingBox.BottomRightCorner.Longitude < coordinate.Longitude {
			boundingBox.BottomRightCorner.Longitude = coordinate.Longitude
		}
	}
	return boundingBox
}

func (cs Coordinates) coordinatesToText() string {
	points := make([]string, 0, len(cs.Coordinates))
	for _, c := range cs.Coordinates {
		points = append(points, fmt.Sprintf("%f %f", c.Latitude, c.Longitude))
	}
	return fmt.Sprintf("'POLYGON((%s))'", strings.Join(points, ","))
}
