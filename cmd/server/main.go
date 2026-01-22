package main

import (
	"fmt"
	"log"
	"polyagent-backend/configs"
)

const configPath = "configs/config.yaml"

func main() {
	// load configuration, initialize services, middleware, and routes here

	//load configuration (configPath)
	conf, err := configs.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	fmt.Printf("Loaded config: %+v\n", conf)
	//initialize database

	//initialize services
	// 将数据库实例传给 service，实现“解耦”
	// userSvc := service.NewUserService(db)
	// orderSvc := service.NewOrderService(db)

	//initialize middleware

	//initialize routes

	// Start HTTP server

	// http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
	// 	fmt.Fprintf(w, "PolyAgent Backend is running")
	// })

	// port := ":8080"
	// log.Printf("Server starting on port %s", port)
	// if err := http.ListenAndServe(port, nil); err != nil {
	// 	log.Fatal(err)
	// }
}
