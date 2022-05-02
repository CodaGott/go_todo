package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
	"context"
	"os"
	"os/signal"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/thedevsaddam/renderer"
	"gopkg.in/mgo.v2/bson"
	mgo "gopkg.in/mgo.v2"
)

var render = *renderer.Render
var db = *mgo.Database

const (
	hostName	string = "localhost:27017"
	dbName		string = "demo_todo"
	collectionName string = "todo"
	port		string = ":9000"
)

type (
	todoModel struct {
		ID 		bson.ObjectId `bson:"id,omitempty"`
		Title string `bson:"title"`
		Completed bool `bson:"completed"`
		CreatedAt time.Time `bson:"createdAt"`
	}

	todo struct {
		ID string `json:"id"`
		Title string `json:"title"`
		Completed bool `json:"completed"`
		CreatedAt time.Time `json:"created_at"`
	}
)

func init() {
	render = renderer.New()
	sess, err:=mgo.Dial(hostName)
	checkErr(err)
	sess.SetMode(mgo.Monotonic, true)
	db = sess.DB(dbName)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	err := render.Template(w, http.StatusOK, []string{"static/home.tpl"}, nil)
	checkErr(err)
}

func fetchTodos(w http.ResponseWriter, r *http.Request)  {
	todos := []todoModel{}
	if err := db.C(collectionName).Find(bson.M{}).All(&todos);
	err != nil {
		render.JSON(w, http.StatusProcessing, renderer.M{
			"Message": "Failed to fetch todo",
			"error":err,
		})
		return
	}
	todoList := []todo{}

	for _, t := range todos {
		todoList = append(todoList, todo{
			ID: t.ID.Hex(),
			Title: t.Title,
			Completed: t.Completed,
			CreatedAt: t.CreatedAt,
		})
	}
	render.JSON(w, http.StatusOK, renderer.M{
		"data": todoList,
	})
}

func createTodo(w http.ResponseWriter, r *http.Request) {
	var t todo
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		render.JSON(w, http.StatusProcessing, err)
		return
	}
	if t.Title == "" {
		render.JSON(w, http.StatusBadRequest, renderer.M{
			"Message": "Title required",
		})
		return
	}
	tModel := todoModel{
		ID: bson.NewObjectId(),
		Title: t.Title,
		Completed : false,
		CreatedAt: time.Now(),
	}
	if err:=db.C(collectionName).Insert(&tModel); err != nil {
		render.JSON(w, http.StatusProcessing, renderer.M{
			"Message": "Failed to save your todo",
			"error": err,
		})
		return
	}
	render.JSON(w, http.StatusCreated, renderer.M{
		"Message": "Todo created sucessfully",
		"todo_id": tModel.ID.Hex(),
	})
}

func deleteTodo(w http.ResponseWriter, r *http.Request) {
	id  := strings.TrimSpace(chi.URLParam(r, "id"))

	if !bson.IsObjectIdHex(id) {
		render.JSON(w, http.StatusBadRequest, renderer.M{
			"Message": "Invalid id provided",
		})
		return
	}
	if err := db.C(collectionName).RemoveId(bson.ObjectIdHex(id)); err != nil {
		render.JSON(w, http.StatusProcessing, renderer.M{
			"Message": "Failed to delete",
			"error": err,
		})
		return
	}
	render.JSON(w, http.StatusOK, renderer.M{
		"Message": "Todo deleted successfully",
	})
}

func updateTodo(w http.ResponseWriter, r *http.Request)  {
	id := strings.TrimSpace(chi.URLParam(r, "id"))

	if !bson.IsObjectIdHex(id){
		render.JSON(w, http.StatusBadRequest, renderer.M{
			"Message": "Invalid id",
		})
		return
	}

	var t todo

	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		render.JSON(w, http.StatusProcessing, err)
		return
	}

	if t.Title == "" {
		render.JSON(w, http.StatusBadRequest, renderer.M{
			"Message" : "the title field id required",
		})
		return
	}

	if err := db.C(collectionName).
		Update(
			bson.M{"_id": bson.ObjectIdHex(id)},
			bson.M{"title": t.Title, "completed": t.Completed},);
	err != nil {
		render.JSON(w, http.StatusProcessing, renderer.M{
			"Message": "Failed to update todo",
			"error": err,
		})
		return
	}
	render.JSON(w, http.StatusOK, renderer.M{
		"message": "Todo updated successfully",
	})
}

func main() {
	stopChannel := make(chan os.Signal)
	signal.Notify(stopChannel, os.Interrupt)

	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Get("/", homeHandler)
	router.Mount("/todo", todoHandlers())

	server := &http.Server{
		Addr: port,
		Handler: router,
		ReadTimeout: 60*time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout: 60 * time.Second,
	}

	go func() {
		log.Println("Listening on port", port)
		if err := server.ListenAndServe(); err != nil {
			log.Printf("listen:%s\n", err)
		}
	}()

	<-stopChannel
	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	server.Shutdown(ctx)
	defer cancel()
		log.Println("Server gracefully stopped!")
}

func todoHandlers() http.Handler {
	groupRouter := chi.NewRouter()
	groupRouter.Group(func(route chi.Router) {
		route.Get("/", fetchTodos)
		route.Post("/", createTodo)
		route.Put("/{id}", updateTodo)
		route.Delete("/{id}", deleteTodo)
	})
	return groupRouter
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
