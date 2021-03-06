package main

import (
	"context"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/99designs/gqlgen/handler"
	"github.com/gin-gonic/gin"
	jsoniter "github.com/json-iterator/go"
	"go.elastic.co/apm"
	"go.elastic.co/apm/module/apmgin"
	"go.elastic.co/apm/module/apmgrpc"
	"go.elastic.co/apm/module/apmhttp"
	"golang.org/x/net/context/ctxhttp"
	"google.golang.org/grpc"

	// "gopkg.in/resty.v1"
	pb "github.com/kinsprite/producttest/pb"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary
var tracingClient = apmhttp.WrapClient(http.DefaultClient)

const prefixV1 = "/api/gin/v1"
const prefixV2 = "/api/gin/v2"

var userServerURL = "http://user-test:80"
var productServerAddress = "producttest:80"

const (
	defaultName = "world"
)

func init() {
	url := os.Getenv("USER_SERVER_URL")

	if url != "" {
		userServerURL = url
	}

	address := os.Getenv("PRODUCT_SERVER_ADDRESS")

	if address != "" {
		productServerAddress = address
	}
}

func fetchProduct(ctxParent context.Context) string {
	// Set up a connection to the server.
	conn, err := grpc.Dial(productServerAddress, grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(apmgrpc.NewUnaryClientInterceptor()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewGreeterClient(conn)

	// Contact the server and print out its response.
	name := defaultName
	if len(os.Args) > 1 {
		name = os.Args[1]
	}
	ctx, cancel := context.WithTimeout(ctxParent, time.Second)
	defer cancel()
	r, err := c.SayHello(ctx, &pb.HelloRequest{Name: name})
	if err != nil {
		log.Fatalf("could not greet: %v", err)
	}
	log.Printf("Greeting: %s", r.Message)

	result := r.Message

	r, err = c.SayHelloAgain(ctx, &pb.HelloRequest{Name: name})
	if err != nil {
		log.Fatalf("could not greet: %v", err)
	}
	log.Printf("Greeting: %s", r.Message)
	return result
}

func main() {
	initDB()
	defer closeDB()

	engine := gin.New()
	engine.Use(apmgin.Middleware(engine))

	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "OK",
		})
	})

	v1 := engine.Group(prefixV1)

	v1.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	v1.POST("/query", graphqlHandler())
	v1.GET("/", playgroundHandler())

	v2 := engine.Group(prefixV2)

	v2.GET("/productsDetails", func(c *gin.Context) {
		req := c.Request
		resp, err := ctxhttp.Get(req.Context(), tracingClient, userServerURL+"/api/user/v1/userInfoBySession")

		if err != nil {
			apm.CaptureError(req.Context(), err).Send()
			log.Println("ERROR   request user info")
			c.AbortWithError(http.StatusInternalServerError, errors.New("failed to query backend"))
			return
		}

		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			log.Println("ERROR   reading user info")
			return
		}

		var userInfo UserInfo
		json.Unmarshal(body, &userInfo)

		productMsg := fetchProduct(req.Context())

		c.JSON(http.StatusOK, gin.H{
			"message":    "all products' details",
			"userId":     userInfo.ID,
			"userName":   userInfo.Name,
			"userEmail":  userInfo.Email,
			"productMsg": productMsg,
		})
	})

	v2.POST("/createUser", func(c *gin.Context) {
		req := c.Request
		resp, err := ctxhttp.Post(
			req.Context(),
			tracingClient,
			userServerURL+"/api/user/v1/new",
			req.Header.Get("Content-Type"),
			c.Request.Body,
		)

		if err != nil {
			apm.CaptureError(req.Context(), err).Send()
			log.Println("ERROR   create user info, ", err)
			c.AbortWithError(http.StatusInternalServerError, errors.New("failed to query backend"))
			return
		}

		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			log.Println("ERROR   reading user info")
			return
		}

		c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
	})

	v2.POST("/mongo", func(c *gin.Context) {
		c.JSON(http.StatusOK, mongoDemo())
	})

	// v2.GET("/userInfo", func(c *gin.Context) {
	// 	resp, err := resty.R().Get(userServerURL + "/api/user/v1/userInfoBySession")

	// 	if err != nil {
	// 		log.Println("ERROR   request user info")
	// 		return
	// 	}

	// 	var userInfo UserInfo
	// 	json.Unmarshal(resp.Body(), &userInfo)

	// 	c.JSON(http.StatusOK, gin.H{
	// 		"userId":   userInfo.ID,
	// 		"userName": userInfo.Name,
	// 	})
	// })

	engine.Run(":8080") // 监听并在 0.0.0.0:8080 上启动服务
}

// Defining the Graphql handler
func graphqlHandler() gin.HandlerFunc {
	// NewExecutableSchema and Config are in the generated.go file
	// Resolver is in the resolver.go file
	h := handler.GraphQL(NewExecutableSchema(Config{Resolvers: &Resolver{}}))

	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// Defining the Playground handler
func playgroundHandler() gin.HandlerFunc {
	h := handler.Playground("GraphQL", prefixV1+"/query")

	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}
