package main

func main() {
	config := ParseConfig()
	app := NewApp(config)
	app.Run()
}
