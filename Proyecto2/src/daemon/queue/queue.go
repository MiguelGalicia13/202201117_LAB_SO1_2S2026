package queue

import (
	"errors"
	"os/exec"
	"strings"
	"log"
)
// Node representa un nodo individual en la cola con apuntadores
type Node struct {
	Data Container
	Next *Node
}

type Queue struct {
	head    *Node
	tail    *Node
	size    int
	maxSize int
}

type Container struct {
	PID    string
	Name   string
	CMDL   string
	CPU    float64
	Memory float64
	VSZ	   float64
	RSS	   float64
	Status string
}

func NewQueue(maxSize int) *Queue {
	return &Queue{
		head:    nil,
		tail:    nil,
		size:    0,
		maxSize: maxSize,
	}
}

// Enqueue agrega un contenedor al final de la cola (tail).
// Si la cola alcanza su capacidad máxima, realiza Dequeue() del contenedor más antiguo antes de insertar.
func (q *Queue) Enqueue(contenedor Container) {
	if q.size == q.maxSize {
		_ = q.Dequeue()
	}
	newNode := &Node{
		Data: contenedor,
		Next: nil,
	}
	if q.tail != nil {
		q.tail.Next = newNode
	} else {
		q.head = newNode
	}
	q.tail = newNode
	q.size++
}

// Dequeue retira y termina (kill) el contenedor más antiguo al frente de la cola (head).
func (q *Queue) Dequeue() error {
	if q.head == nil {
		return errors.New("la cola está vacía")
	}
	container := q.head.Data

	// Matar el proceso del contenedor
	cmd := exec.Command("sh" ,"-c", "docker stop "+container.PID)
	output, err := cmd.Output()
	if err != nil {
		log.Fatal("[ERROR] PROCESS COULDNT BE TERMINATED ", err.Error())
		return err
	}
	cmd = exec.Command("sh", "-c", "docker rm "+container.PID)
	output, err = cmd.Output()
	if err !=nil{
		log.Fatal("[ERROR] CONTAINER COULDNT BE REMOVED ", err.Error())
		return err
	}
	log.Println("[LOG] CONTAINER SUCCESFULLY STOPED ",strings.TrimSpace(string(output)))

	q.head = q.head.Next
	if q.head == nil {
		q.tail = nil
	}
	q.size--
	return err
}

// Size retorna el tamaño actual de la cola.
func (q *Queue) Size() int {
	return q.size
}

// MaxSize retorna la capacidad máxima de la cola.
func (q *Queue) MaxSize() int {
	return q.maxSize
}

// IsEmpty indica si la cola está vacía.
func (q *Queue) IsEmpty() bool {
	return q.head == nil
}

// Peek retorna una referencia al contenedor al frente de la cola sin sacarlo.
func (q *Queue) Peek() (*Container, error) {
	if q.head == nil {
		return nil, errors.New("la cola está vacía")
	}
	return &q.head.Data, nil
}