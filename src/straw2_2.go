package main

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"math"
	"math/rand"
	"time"
)

type Item struct {
	id     uint32
	weight uint32
}

type Bucket struct {
	weight uint32
	items  []Item
}

func NewBucket() *Bucket {
	return &Bucket{items: make([]Item, 0)}
}

func (bucket *Bucket) AddItem(id, weight uint32) {
	bucket.weight += weight
	bucket.items = append(bucket.items, Item{id: id, weight: weight})
}

func Hash(x uint32) uint32 {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, uint32(x))
	return crc32.ChecksumIEEE(data)
}

func Hash2(x, id uint32) uint32 {
	data := make([]byte, 8)
	binary.BigEndian.PutUint32(data, uint32(x))
	binary.BigEndian.PutUint32(data[4:], uint32(x))
	return crc32.ChecksumIEEE(data)
}

func (bucket *Bucket) Select(x uint32) uint32 {
	max_item_id := uint32(0)
	max_draw := -math.MaxFloat64
	for id, item := range bucket.items {
		draw := -math.MaxFloat64
		if item.weight != 0 {
			h := Hash(x * uint32(id+100))
			//h := Hash2(x, uint32(id))
			//h &= 0xffff
			//draw = math.Log(float64(h)/65536.0) / float64(item.weight)
			draw = math.Log(float64(h)/4294967296.0) / float64(item.weight)
		}

		if draw > max_draw {
			max_item_id = item.id
			max_draw = draw
		}
	}
	return max_item_id
}

type DistributeStat struct {
	averageBias        float64
	averageBiasPercent float64
	maxBias            uint32
	maxBiasPercent     float64
}

func (stat *DistributeStat) String() string {
	str := fmt.Sprintf("averageBias        = %2.2f\n", stat.averageBias)
	str += fmt.Sprintf("averageBiasPercent = %2.2f%%\n", stat.averageBiasPercent*100)
	str += fmt.Sprintf("maxBias            = %d\n", stat.maxBias)
	str += fmt.Sprintf("maxBiasPercent     = %2.2f%%\n", stat.maxBiasPercent*100)
	return str
}

type MigrateStat struct {
	migrateIn  uint32
	migrateOut uint32
}

type Node struct {
	id     uint32
	weight uint32
	data   []uint32
}

func (node *Node) Clone() *Node {
	ret := &Node{id: node.id, weight: node.weight}
	ret.data = make([]uint32, 0)
	for _, v := range node.data {
		ret.data = append(ret.data, v)
	}
	return ret
}

type Nodes struct {
	id     uint32
	weight uint32
	nodes  []*Node
	total  uint32
}

func NewNodes(id uint32) *Nodes {
	nodes := &Nodes{id: id}
	nodes.nodes = make([]*Node, 0)
	return nodes
}

func (nodes *Nodes) Clone() *Nodes {
	ret := &Nodes{id: nodes.id, weight: nodes.weight, total: nodes.total}
	ret.nodes = make([]*Node, 0)
	for _, v := range nodes.nodes {
		ret.nodes = append(ret.nodes, v.Clone())
	}
	return ret
}

func (nodes *Nodes) AddNode(id, weight uint32) {
	nodes.weight += weight
	nodes.nodes = append(nodes.nodes, &Node{id: id, weight: weight, data: make([]uint32, 0)})
}

func (nodes *Nodes) AddNodeData(id, data uint32) {
	nodes.nodes[id-1].data = append(nodes.nodes[id-1].data, data)
	nodes.total++
}

func (nodes *Nodes) ChangeWeight(id, weight uint32) {
	delta := weight - nodes.nodes[id-1].weight
	nodes.weight += delta
	nodes.nodes[id-1].weight += delta
}

func (nodes *Nodes) String() string {
	str := fmt.Sprintf("weight = %d\n", nodes.weight)

	for _, node := range nodes.nodes {
		str += fmt.Sprintf("[%d]: weight = %d, counts = %d\n", node.id, node.weight, len(node.data))
	}
	return str
}

func (nodes *Nodes) PrintCount(name, sub_name string) string {
	str := fmt.Sprintf("%s[%d]: total = %d\n", name, nodes.id, nodes.total)

	for _, node := range nodes.nodes {
		str += fmt.Sprintf("    %s[%d]: counts = %d\n", sub_name, node.id, len(node.data))
	}
	return str
}

func (nodes *Nodes) Stat(standard uint32) (stat *DistributeStat) {
	stat = &DistributeStat{}

	var bias uint32

	for _, node := range nodes.nodes {
		node_num := uint32(len(node.data))

		if node_num > standard {
			bias = node_num - standard
		} else {
			bias = standard - node_num
		}
		biasPercent := float64(bias) / float64(node_num)
		if bias > stat.maxBias {
			stat.maxBias = bias
		}

		if biasPercent > stat.maxBiasPercent {
			stat.maxBiasPercent = biasPercent
		}

		stat.averageBias += float64(bias)
		stat.averageBiasPercent += biasPercent
	}

	stat.averageBias /= float64(len(nodes.nodes))
	stat.averageBiasPercent /= float64(len(nodes.nodes))

	return stat
}

func BuildNodes(id, total uint32) *Nodes {
	nodes := NewNodes(id)

	for i := uint32(0); i < total; i++ {
		nodes.AddNode(uint32(i+1), 2)
	}

	//nodes.ChangeWeight(3, 6)
	//nodes.ChangeWeight(7, 3)

	return nodes
}

func BuildBucket(nodes *Nodes) *Bucket {
	bucket := NewBucket()

	for _, node := range nodes.nodes {
		bucket.AddItem(node.id, node.weight)
	}
	return bucket
}

type Device struct {
	mgs   []*Nodes
	total uint32
}

func (device *Device) Clone() *Device {
	ret := &Device{total: device.total}
	ret.mgs = make([]*Nodes, 0)
	for _, mg := range device.mgs {
		ret.mgs = append(ret.mgs, mg.Clone())
	}
	return device
}

func (device *Device) String() string {
	str := fmt.Sprintf("Device: total = %d\n", device.total)
	for _, mg := range device.mgs {
		str += mg.PrintCount("MG", "PE")
	}
	return str
}

func BuildDevice(mg_num, pe_num uint32) *Device {
	device := &Device{total: 0}
	device.mgs = make([]*Nodes, 0)
	for i := uint32(0); i < mg_num; i++ {
		mg := BuildNodes(uint32(i+1), pe_num)
		mg.weight = pe_num
		device.mgs = append(device.mgs, mg)
	}
	return device
}

func Run() {

	total_nodes := uint32(10)

	nodes1 := BuildNodes(1, total_nodes)
	nodes2 := BuildNodes(2, total_nodes)

	bucket1 := BuildBucket(nodes1)

	user_num := uint32(50000)

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := uint32(0); i < user_num; i++ {
		//x := i
		x := r.Uint32()
		id1 := bucket1.Select(x)
		id2 := x%total_nodes + 1
		nodes1.AddNodeData(id1, x)
		nodes2.AddNodeData(id2, x)
	}

	stat1 := nodes1.Stat(user_num / total_nodes)
	stat2 := nodes2.Stat(user_num / total_nodes)

	fmt.Printf("nodes for straw2:\n%s", nodes1)
	fmt.Printf("nodes for mod:\n%s", nodes2)
	fmt.Printf("stat for straw2:\n%s", stat1)
	fmt.Printf("\n")
	fmt.Printf("stat for mod:\n%s", stat2)
}

func main() {
	device := BuildDevice(10, 2)
	fmt.Println(device)
	//fmt.Printf("device: \n", device)
	Run()
}
