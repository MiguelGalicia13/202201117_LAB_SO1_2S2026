package queue

import (
	"errors"
	"os/exec"
	"strconv"
	"fmt"
)
type Queue struct{
	size int
	maxSize int
	containers []Container
}

type Container struct{
	PID int
	Name string
	CPU float64
	Memory float64
	Status string
}

func NewQueue(max_size int) *Queue{
	return &Queue{
		size: 0,
		maxSize: max_size,
		containers: []Container{},
	}
}

func (q *Queue) addToqueue (contenedor Container){
	if (q.size == q.maxSize){
		q.dequeue()
	}
	q.containers = append(q.containers, contenedor)
	q.size ++;
}

func (q *Queue) dequeue()(error){
	var conteiner Container
	if(len(q.containers) == 0){
		return errors.New("la cola está vacía")
	}
	conteiner = q.containers[0]
	// Matar el proceso del contenedor
	cmd := exec.Command("kill", strconv.Itoa(conteiner.PID))
	output, err := cmd.Output()
	if err != nil {
			fmt.Println("Error al ejecutar:", err)
			return err
		}
	fmt.Println(string(output))
	q.containers = q.containers[1:]
	return nil
}