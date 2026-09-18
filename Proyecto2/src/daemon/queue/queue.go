package queue

import (
	"errors"
	"syscall"
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
	PID    int
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
	log.Printf("[LOG] CONTAINER ADDED TO QUEUE %d\n", newNode.Data.PID)
}

// Dequeue retira y termina (kill) el contenedor más antiguo al frente de la cola (head).
func (q *Queue) Dequeue() error {
	if q.head == nil {
		return errors.New("la cola está vacía")
	}
	container := q.head.Data

	q.head = q.head.Next
	if q.head == nil {
		q.tail = nil
	}
	q.size--

	// Matar el proceso del contenedor
	err := killProcess(container.PID)
	if err != nil {
		return err
	}
	log.Println("[LOG] CONTAINER SUCCESFULLY STOPED ")
	container.Status = "stoped"

	return nil
}

// PurgeInactive elimina de la cola cualquier proceso que ya no figure en el mapa de procesos vivos del kernel
func (q *Queue) PurgeInactive(activePIDs map[int]bool) {
	// 1. Limpiar nodos inactivos que estén al inicio (head)
	for q.head != nil && !activePIDs[q.head.Data.PID] {
		log.Printf("[QUEUE] CONTAINER PID %d TERMINATED EXTERNALLY, PURGED FROM QUEUE\n", q.head.Data.PID)
		q.head = q.head.Next
		q.size--
	}

	if q.head == nil {
		q.tail = nil
		return
	}

	// 2. Limpiar nodos inactivos en el medio o final
	current := q.head
	for current.Next != nil {
		if !activePIDs[current.Next.Data.PID] {
			log.Printf("[QUEUE] CONTAINER PID %d TERMINATED EXTERNALLY, PURGED FROM QUEUE\n", current.Next.Data.PID)
			current.Next = current.Next.Next
			q.size--
		} else {
			current = current.Next
		}
	}
	q.tail = current
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

func killProcess(pid int)error{
	err := syscall.Kill(pid, syscall.SIGKILL)
	if err !=nil{
		log.Println("[ERROR] PROCES COULD NOT BE TERMINATED", err.Error())
		return err
	}
	log.Println("[LOG] PROCESS SUCCESFULLY TERMINATED ", pid)
	return nil
}

func (q *Queue) StopAll() {
    for q.head != nil {
        _ = q.Dequeue()                                                                                                                       
    }
}

func (q *Queue) Contains(pid int)bool{
	current :=q.head
	for current != nil{
		if current.Data.PID == pid{
			return true
		}
		current = current.Next
	}
	return false
}