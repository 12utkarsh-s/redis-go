package core

import (
	"fmt"
	"log"
	"os"
	"redis-go/config"
	"strings"
)

func dump(key string, obj *Object, file *os.File) {
	cmd := fmt.Sprintf("SET %s %s", key, obj.Value)
	tokens := strings.Split(cmd, " ")
	file.Write(Encode(tokens, false))
}

func DumpAllAOF() {
	file, err := os.OpenFile(config.AOFFile, os.O_WRONLY|os.O_CREATE, os.ModeAppend)
	if err != nil {
		log.Println("error ", err)
		return
	}

	log.Println("rewriting AOF file at", config.AOFFile)
	for key, obj := range store {
		dump(key, obj, file)
	}
	log.Println("AOF file rewrite complete")
}
