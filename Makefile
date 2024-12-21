# Имя выходного файла  
BINARY=main.exe  

# Компиляция  
build:  
	go build -o bin/$(BINARY) cmd/main.go

# Запуск с параметром  
run:   
	./bin/$(BINARY)

# Удаление скомпилированного файла  
clean:  
	rm -rf ./bin/$(BINARY)