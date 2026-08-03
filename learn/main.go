package main // 入口包，类似 JS 里的入口文件；必须叫 main 才能 go run

import (
	"encoding/json" // 相当于 JSON.parse / JSON.stringify
	"embed"         // 把静态文件“打包进”二进制，类似 webpack 内联资源
	"fmt"           // 打印：Println / Printf
	"io/fs"
	"log"
	"net/http" // 内置 HTTP 服务器，不用 Express
	"strconv"
	"sync" // 锁，多请求同时改内存数据时防脏写（前端一般遇不到）
	"time"
)

// //go:embed 是编译器指令（不是普通注释！）
// 作用：把 static/ 目录打进程序里，部署时不用再单独拷 HTML
//go:embed static/*
var staticFS embed.FS

// type + struct ≈ TypeScript 的 interface / type
// 后面的 `json:"id"` 叫 struct tag，告诉 JSON 库字段怎么命名
// （TS 里类似 @JsonProperty / class-transformer 的装饰器）
type Todo struct {
	ID        int       `json:"id"`         // 导出字段必须大写开头；JSON 里变小写靠 tag
	Title     string    `json:"title"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"created_at"`
}

// Store 内存里的“数据库”
type Store struct {
	mu     sync.RWMutex   // 读写锁：读多写少时比普通锁更高效
	todos  map[int]*Todo  // map ≈ JS 的 Map / 普通对象；*Todo 是指针（见下）
	nextID int
}

func NewStore() *Store {
	// &Store{...}：取地址，返回指针。≈ JS 里 new 出一个对象并传来传去引用
	// make(map[...])：创建空 map，必须 make / 字面量，不能只用 var（否则是 nil）
	s := &Store{
		todos:  make(map[int]*Todo),
		nextID: 1,
	}
	s.Create("学习 Go 语言")
	todo := s.Create("写一个 CRUD Demo")

	// %d 只能打 int；s 是整个 Store，要用 %v / %+v
	fmt.Printf("刚创建的 id=%d\n", todo.ID) // 打数字字段
	fmt.Printf("todo=%+v\n", todo)         // 打结构体（带字段名）
	fmt.Printf("store nextID=%d, 条数=%d\n", s.nextID, len(s.todos))
	return s
}

// func (s *Store) List() —— 这是“方法”
// (s *Store) 叫接收者，类似 TS class 里的 this；*Store 表示改的是同一份数据
// []*Todo：Todo 指针的切片（slice ≈ 可变长度数组 / JS Array）
func (s *Store) List() []*Todo {
	s.mu.RLock()         // 加读锁
	defer s.mu.RUnlock() // defer：函数返回前一定执行，类似 finally / 自动 unlock

	// make([]*Todo, 0, len)：长度 0、容量 len，先占好空间再 append
	list := make([]*Todo, 0, len(s.todos))
	// for range ≈ for...of；_ 丢掉 key（id），只要 value
	for _, t := range s.todos {
		list = append(list, t) // append ≈ Array.push，但返回新切片要接回来
	}
	return list
}

// Go 可以多返回值！(*Todo, bool) ≈ TS 的 [Todo | null, boolean]
// 常用套路：值 + 是否成功（ok）
func (s *Store) Get(id int) (*Todo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// map 取值：t, ok := m[key]
	// ok=false 表示没有这个 key（不会像 JS 那样得到 undefined 还继续用）
	t, ok := s.todos[id]
	return t, ok
}

func (s *Store) Create(title string) *Todo {
	s.mu.Lock() // 写锁：独占
	defer s.mu.Unlock()
	todo := &Todo{
		ID:        s.nextID,
		Title:     title,
		Done:      false,
		CreatedAt: time.Now().UTC(),
	}
	s.todos[s.nextID] = todo
	s.nextID++
	return todo
}

func (s *Store) Update(id int, title string, done bool) (*Todo, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	todo, ok := s.todos[id]
	if !ok {
		return nil, false // nil ≈ null / undefined（针对指针、slice、map、interface）
	}
	todo.Title = title
	todo.Done = done
	return todo, true
}

func (s *Store) Delete(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	// _, ok：只要 ok，不要值；_ 是“故意丢掉”的占位符
	if _, ok := s.todos[id]; !ok {
		return false
	}
	delete(s.todos, id) // 内置函数，删 map 的 key
	return true
}

type Server struct {
	store *Store
	mux   *http.ServeMux // 路由器，类似 express.Router()
}

func NewServer(store *Store) *Server {
	s := &Server{store: store, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) routes() {
	// Go 1.22+ 路由可写成 "METHOD /path/{param}"，像很多现代框架一样
	s.mux.HandleFunc("GET /api/todos", s.handleList)
	s.mux.HandleFunc("GET /api/todos/{id}", s.handleGet)
	s.mux.HandleFunc("POST /api/todos", s.handleCreate)
	s.mux.HandleFunc("PUT /api/todos/{id}", s.handleUpdate)
	s.mux.HandleFunc("DELETE /api/todos/{id}", s.handleDelete)

	// fs.Sub：从 embed 出来的文件系统里切出 static 子目录当网站根
	web, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatal(err) // 打印错误并退出进程
	}
	// FileServer：静态文件服务，≈ express.static('static')
	s.mux.Handle("GET /", http.FileServer(http.FS(web)))
}

// ServeHTTP 让 *Server 实现 http.Handler 接口
// （Go 的“接口”是隐式的：方法签名对上就算实现，不用 implements 关键字）
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent) // 204
		return
	}

	log.Printf("%s %s", r.Method, r.URL.Path) // %s 类似 console.log 占位符
	s.mux.ServeHTTP(w, r)
}

// _ *http.Request：参数名写成 _，表示“这个参数我用不到，但签名必须留着”
func (s *Server) handleList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.List())
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	todo, found := s.store.Get(id)
	if !found {
		// map[string]string{...}：匿名 map 字面量，直接当 JSON 对象返回
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "未找到"})
		return
	}
	writeJSON(w, http.StatusOK, todo)
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	// 匿名 struct：只用一次的类型，不必单独 type Body struct
	// 作用 ≈ const body = await req.json() as { title: string }
	var body struct {
		Title string `json:"title"`
	}
	// Decode(&body)：传入指针才能真正写入 body；& 取地址
	// := 短变量声明：自动推断类型，≈ const err = ...
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title 不能为空"})
		return
	}
	writeJSON(w, http.StatusCreated, s.store.Create(body.Title))
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var body struct {
		Title string `json:"title"`
		Done  bool   `json:"done"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title 不能为空"})
		return
	}
	todo, found := s.store.Update(id, body.Title, body.Done)
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "未找到"})
		return
	}
	writeJSON(w, http.StatusOK, todo)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if !s.store.Delete(id) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "未找到"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// (int, bool) 多返回值：解析成功的 id + 是否 ok
func parseID(w http.ResponseWriter, r *http.Request) (int, bool) {
	// PathValue("id")：取路由里的 {id}，类似 req.params.id
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的 ID"})
		return 0, false
	}
	return id, true
}

// any ≈ TypeScript 的 any / unknown（Go 1.18+ 对 interface{} 的别名）
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status) // 注意：先写 Header，再 WriteHeader，再写 body
	// _ = ... 故意忽略 Encode 的 error（demo 简化）；生产环境应处理
	_ = json.NewEncoder(w).Encode(v)
}

func main() {
	addr := ":8080" // 前面省略 host，表示监听所有网卡的 8080
	server := NewServer(NewStore())

	log.Printf("CRUD Demo 已启动: http://localhost%s", addr)
	log.Printf("增 POST   /api/todos")
	log.Printf("删 DELETE /api/todos/{id}")
	log.Printf("改 PUT    /api/todos/{id}")
	log.Printf("查 GET    /api/todos  |  /api/todos/{id}")

	// ListenAndServe 会一直阻塞（像 app.listen），直到出错
	if err := http.ListenAndServe(addr, server); err != nil {
		log.Fatal(err)
	}
}
