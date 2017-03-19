package main

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"math"
	"math/rand"
	"time"
)

const crush_hash_seed uint32 = uint32(1315423911)

func crush_hashmix(a, b, c uint32) (uint32, uint32, uint32) {
	a = a - b
	a = a - c
	a = a ^ (c >> 13)
	b = b - c
	b = b - a
	b = b ^ (a << 8)
	c = c - a
	c = c - b
	c = c ^ (b >> 13)
	a = a - b
	a = a - c
	a = a ^ (c >> 12)
	b = b - c
	b = b - a
	b = b ^ (a << 16)
	c = c - a
	c = c - b
	c = c ^ (b >> 5)
	a = a - b
	a = a - c
	a = a ^ (c >> 3)
	b = b - c
	b = b - a
	b = b ^ (a << 10)
	c = c - a
	c = c - b
	c = c ^ (b >> 15)
	return a, b, c
}

func crush_hash32_rjenkins1_2(a, b uint32) uint32 {
	hash := crush_hash_seed ^ a ^ b
	x := uint32(231232)
	y := uint32(1232)
	a, b, hash = crush_hashmix(a, b, hash)
	x, a, hash = crush_hashmix(x, a, hash)
	b, y, hash = crush_hashmix(b, y, hash)
	return hash
}

func crush_hash32_rjenkins1_3(a, b, c uint32) uint32 {
	hash := crush_hash_seed ^ a ^ b ^ c
	x := uint32(231232)
	y := uint32(1232)
	a, b, hash = crush_hashmix(a, b, hash)
	c, x, hash = crush_hashmix(c, x, hash)
	y, a, hash = crush_hashmix(y, a, hash)
	b, x, hash = crush_hashmix(b, x, hash)
	y, c, hash = crush_hashmix(y, c, hash)
	return hash
}

func Hash(x uint32) uint32 {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, uint32(x))
	return crc32.ChecksumIEEE(data)
}

func Hash2(x, mg_id uint32) uint32 {
	data := make([]byte, 8)
	binary.BigEndian.PutUint32(data, uint32(x))
	binary.BigEndian.PutUint32(data[4:], uint32(mg_id))
	return crc32.ChecksumIEEE(data)
}

func Hash3(x, mg_id, pe_id uint32) uint32 {
	data := make([]byte, 12)
	binary.BigEndian.PutUint32(data, uint32(x))
	binary.BigEndian.PutUint32(data[4:], uint32(mg_id))
	binary.BigEndian.PutUint32(data[8:], uint32(pe_id))

	return crc32.ChecksumIEEE(data)
}

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

func (self *Bucket) Clone(bucket *Bucket) {
	bucket.weight = self.weight
	bucket.items = make([]Item, len(self.items))
	copy(bucket.items, self.items)
}

func (self *Bucket) AddItem(id, weight uint32) {
	self.weight += weight
	self.items = append(self.items, Item{id: id, weight: weight})
}

func (self *Bucket) DelItem(index uint32) {
	self.weight -= self.items[index].weight
	self.items = append(self.items[:index], self.items[index+1:]...)
}

func (self *Bucket) SetWeight(index, weight uint32) {
	old_weight := self.items[index].weight
	if weight >= old_weight {
		self.weight += weight - old_weight
	} else {
		self.weight -= old_weight - weight
	}

	self.items[index].weight = weight
}

func (bucket *Bucket) Select(x uint32) uint32 {
	max_item_id := uint32(0)
	max_draw := -math.MaxFloat64
	for _, item := range bucket.items {
		draw := -math.MaxFloat64
		id := item.id
		weight := item.weight
		if weight != 0 {
			//h := Hash(x * uint32(id+100))
			//fmt.Println("id =", id)
			//h := Hash2(x, uint32(id))
			h := crush_hash32_rjenkins1_2(x, uint32(id))
			//fmt.Printf("x = %d, mg_id = %d, h = %d\n", x, id, h)
			//fmt.Println("h =", h)
			//h &= 0xffff
			//draw = math.Log(float64(h)/65536.0) / float64(item.weight)
			draw = math.Log(float64(h)/4294967296.0) / float64(weight)
		}

		if draw > max_draw {
			max_item_id = id
			max_draw = draw
		}
	}
	//fmt.Println("mg_id =", max_item_id)
	return max_item_id
}

func (bucket *Bucket) Select2(mg_id, x uint32) uint32 {
	max_item_id := uint32(0)
	max_draw := -math.MaxFloat64
	for _, item := range bucket.items {
		draw := -math.MaxFloat64
		id := item.id
		weight := item.weight
		if weight != 0 {
			//h := Hash(x * uint32(id+100))
			//fmt.Println("id =", id)
			//h := Hash(x * uint32(id+100) * (mg_id + 200))
			//h := Hash2(x*uint32(id+100), mg_id)
			//h := Hash3(x, mg_id, uint32(id))
			//h := crush_hash32_rjenkins1_2(x, (mg_id+100)*uint32(id))
			h := crush_hash32_rjenkins1_3(x, mg_id, uint32(id))
			//fmt.Printf("y = %d, mg_id = %d, h = %d\n", x, id, h)
			//fmt.Println("h =", h)
			//h &= 0xffff
			//draw = math.Log(float64(h)/65536.0) / float64(item.weight)
			draw = math.Log(float64(h)/4294967296.0) / float64(weight)
		}

		if draw > max_draw {
			max_item_id = id
			max_draw = draw
		}
	}
	//fmt.Println("pe_id =", max_item_id)
	return max_item_id
}

type DistributeStat struct {
	averageBias        float64
	averageBiasPercent float64
	maxBias            uint32
	maxBiasPercent     float64
}

func (self *DistributeStat) String() string {
	str := fmt.Sprintf("平均偏差（个）= %2.2f, ", self.averageBias)
	str += fmt.Sprintf("平均偏差百分比（%%）= %2.2f%%, ", self.averageBiasPercent*100)
	str += fmt.Sprintf("最大偏差（个）= %d, ", self.maxBias)
	str += fmt.Sprintf("最大偏差百分比（%%）= %2.2f%%", self.maxBiasPercent*100)
	return str
}

type MigrateStat struct {
	migrateIn  uint32
	migrateOut uint32
}

func (self *MigrateStat) String() string {
	return fmt.Sprintf("迁入 = %d, 迁出 = %d", self.migrateIn, self.migrateOut)
}

func (self *MigrateStat) Clear() {
	self.migrateIn = 0
	self.migrateOut = 0
}

type PE struct {
	id       uint32
	weight   uint32
	standard uint32
	data     map[uint32]uint32
	migrate  MigrateStat
}

func (self *PE) ClearData() {
	for k, _ := range self.data {
		delete(self.data, k)
	}
}

func (self *PE) ClearMigrate() {
	self.migrate.Clear()
}

func (self *PE) AddData(data uint32) {
	if _, ok := self.data[data]; ok {
		panic("element exist")
	}
	self.data[data] = data

}

func (self *PE) DelData(data uint32) {
	if _, ok := self.data[data]; !ok {
		panic("element not exist")
	}
	delete(self.data, data)
}

func (self *PE) MigrateInData(data uint32) {
	self.AddData(data)
	self.migrate.migrateIn++
}

func (self *PE) MigrateOutData(data uint32) {
	self.DelData(data)
	self.migrate.migrateOut++
}

func (self *PE) Clone() *PE {
	pe := &PE{id: self.id, weight: self.weight, migrate: self.migrate}
	pe.data = make(map[uint32]uint32)
	for k, v := range self.data {
		pe.data[k] = v
	}

	return pe
}

func (self *PE) ScaleOutMg(device *Device, mg_id uint32) {

	for key, _ := range self.data {
		to_mg_id, to_pe_id := device.Select(key)
		//fmt.Printf("from_mg_id = %d, from_pe_id = %d, to_mg_id = %d, to_pe_id = %d\n", mg_id, self.id, to_mg_id, to_pe_id)
		if to_mg_id != mg_id {
			device.Migrate(mg_id, self.id, to_mg_id, to_pe_id, key)
		}
	}
}

func (self *PE) ScaleInMg(device *Device, mg_id uint32) {

	for key, _ := range self.data {
		to_mg_id, to_pe_id := device.Select(key)
		//fmt.Printf("from_mg_id = %d, from_pe_id = %d, to_mg_id = %d, to_pe_id = %d\n", mg_id, self.id, to_mg_id, to_pe_id)
		if to_mg_id != mg_id {
			device.Migrate(mg_id, self.id, to_mg_id, to_pe_id, key)
		} else {
			fmt.Println("scale in mg error: location not changed")
		}
	}
}

func (self *PE) PrintCount() string {
	return fmt.Sprintf("PE[%d]: counts = %d\n", self.id, len(self.data))
}

func (self *PE) PrintWeight() string {
	return fmt.Sprintf("PE[%d]: weight = %d\n", self.id, self.weight)
}

func (self *PE) PrintData() string {
	str := fmt.Sprintf("PE[%d]: data = [", self.id)
	for k, _ := range self.data {
		str += fmt.Sprintf("%d ", k)
	}
	str += "]\n"
	return str
}

func (self *PE) PrintMigrate() string {
	return fmt.Sprintf("PE[%d]: %s\n", self.id, self.migrate.String())
}

func (self *PE) String() string {
	return fmt.Sprintf("PE[%d]: weight = %d, counts = %d, data = %v\n", self.id, self.weight, len(self.data), self.data)
}

type MG struct {
	id        uint32
	weight    uint32
	total     uint32
	standard  uint32
	pes       []*PE
	stat      DistributeStat
	migrate   MigrateStat
	pe_bucket Bucket
}

func NewMG(mg_id, pe_num, pe_weight uint32) *MG {
	mg := &MG{id: mg_id}
	for i := uint32(0); i < pe_num; i++ {
		mg.AddPe(i+1, pe_weight)
	}
	return mg
}

func (self *MG) GetId(index uint32) uint32 {
	return self.pes[index].id
}

func (self *MG) GetWeight(index uint32) uint32 {
	return self.pes[index].weight
}

func (self *MG) Size() uint32 {
	return uint32(len(self.pes))
}

func (self *MG) GetPeIndex(pe_id uint32) (index uint32) {
	for i, v := range self.pes {
		if pe_id == v.id {
			return uint32(i)
		}
	}
	panic("cannot find pe by pe_id")
}

func (self *MG) ClearData() {
	for _, v := range self.pes {
		v.ClearData()
	}
	self.total = 0
}

func (self *MG) ClearMigrate() {
	self.migrate.Clear()
	for _, v := range self.pes {
		v.ClearMigrate()
	}
}

func (self *MG) ScaleOutMg(device *Device) {
	for _, v := range self.pes {
		v.ScaleOutMg(device, self.id)
	}
}

func (self *MG) ScaleInMg(device *Device) {
	for _, v := range self.pes {
		v.ScaleInMg(device, self.id)
	}
}

func (self *MG) Select(key uint32) (pe_id uint32) {
	return self.pe_bucket.Select2(self.id, key)
}

func (self *MG) AddPe(pe_id, weight uint32) {
	self.weight += weight
	self.pes = append(self.pes, &PE{id: pe_id, weight: weight, data: make(map[uint32]uint32, 0)})
	self.pe_bucket.AddItem(pe_id, weight)
}

func (self *MG) AddData(index, data uint32) {
	self.total++
	self.pes[index].AddData(data)
}

func (self *MG) DelData(pe_index, data uint32) {
	self.pes[pe_index].DelData(data)
}

func (self *MG) MigrateInData(pe_index, data uint32) {
	self.pes[pe_index].MigrateInData(data)
	self.migrate.migrateIn++
	self.total++
}

func (self *MG) MigrateOutData(pe_index, data uint32) {
	self.pes[pe_index].MigrateOutData(data)
	self.migrate.migrateOut++
	self.total--
}

func (self *MG) SetPeWeight(index, weight uint32) {
	old_weight := self.pes[index].weight
	if weight >= old_weight {
		self.weight += weight - old_weight
	} else {
		self.weight -= old_weight - weight
	}

	self.pes[index].weight = weight
	self.pe_bucket.SetWeight(index, weight)
}

func (self *MG) Clone() *MG {
	mg := &MG{id: self.id, weight: self.weight, total: self.total, migrate: self.migrate}
	mg.pes = make([]*PE, 0)
	for _, v := range self.pes {
		mg.pes = append(mg.pes, v.Clone())
	}
	self.pe_bucket.Clone(&mg.pe_bucket)
	return mg
}

func (self *MG) PrintCount() string {
	str := fmt.Sprintf("MG[%d]: total = %d\n", self.id, self.total)

	for _, pe := range self.pes {
		str += fmt.Sprintf("    %s", pe.PrintCount())
	}
	return str
}

func (self *MG) PrintWeight() string {
	str := fmt.Sprintf("MG[%d]: weight = %d\n", self.id, self.weight)

	for _, pe := range self.pes {
		str += fmt.Sprintf("    %s", pe.PrintWeight())
	}
	return str
}

func (self *MG) PrintData() string {
	str := fmt.Sprintf("MG[%d]: total = %d\n", self.id, self.total)

	for _, pe := range self.pes {
		str += fmt.Sprintf("    %s", pe.PrintData())
	}
	return str
}

func (self *MG) PrintStat() string {
	return fmt.Sprintf("MG[%d]: %s\n", self.id, self.stat.String())
}

func (self *MG) PrintMigrate() string {
	str := fmt.Sprintf("MG[%d]: %s\n", self.id, self.migrate.String())

	for _, pe := range self.pes {
		str += fmt.Sprintf("    %s", pe.PrintMigrate())
	}
	return str
}

func (self *MG) String() string {
	str := fmt.Sprintf("MG[%d]: weight = %d, total = %d\n", self.id, self.weight, self.total)

	for _, pe := range self.pes {
		str += fmt.Sprintf("    %s", pe)
	}
	return str
}

func (self *MG) SetStandard(total uint32) {
	pe_standard := total / self.Size()
	for _, pe := range self.pes {
		pe.standard = pe_standard
	}
}

type Device struct {
	id        uint32
	weight    uint32
	total     uint32
	mgs       []*MG
	stat      DistributeStat
	mg_bucket Bucket
}

func NewDevice(mg_num, pe_num, pe_weight uint32) *Device {
	device := &Device{}

	for i := uint32(0); i < mg_num; i++ {
		mg := NewMG(i+1, pe_num, pe_weight)
		device.AddMg(mg)
	}
	return device
}

func (self *Device) GetId(index uint32) uint32 {
	return self.mgs[index].id
}

func (self *Device) GetWeight(index uint32) uint32 {
	return self.mgs[index].weight
}

func (self *Device) Size() uint32 {
	return uint32(len(self.mgs))
}

func (self *Device) GetMgIndex(mg_id uint32) (index uint32) {
	for i, v := range self.mgs {
		if mg_id == v.id {
			return uint32(i)
		}
	}
	panic("cannot find mg by mg_id")
}

func (self *Device) ClearData(data uint32) {
	for _, v := range self.mgs {
		v.ClearData()
	}
	self.total = 0
}

func (self *Device) Select(key uint32) (mg_id, pe_id uint32) {
	mg_id = self.mg_bucket.Select(key)
	//fmt.Println("mg_id =", mg_id)
	mg_index := self.GetMgIndex(mg_id)
	pe_id = self.mgs[mg_index].Select(key)
	return mg_id, pe_id
}

func (self *Device) ClearMigrate() {
	for _, v := range self.mgs {
		v.ClearMigrate()
	}
}

func (self *Device) AddMg(mg *MG) {
	self.weight += mg.weight
	self.total += mg.total
	self.mgs = append(self.mgs, mg)
	self.mg_bucket.AddItem(mg.id, mg.weight)
	//fmt.Println("self.mg_bucket =", self.mg_bucket)
}

func (self *Device) DelMg(mg_id uint32) {
	mg_index := self.GetMgIndex(mg_id)
	mg := self.mgs[mg_index]
	self.weight -= mg.weight
	self.total -= mg.total
	self.mgs = append(self.mgs[:mg_index], self.mgs[mg_index+1:]...)
	//self.mg_bucket.DelItem(mg_index, mg.weight)
	//fmt.Println("self.mg_bucket =", self.mg_bucket)
}

func (self *Device) AddData(mg_index, pe_index, data uint32) {
	self.total++
	self.mgs[mg_index].AddData(pe_index, data)
}

func (self *Device) AddDataById(mg_id, pe_id, data uint32) {
	mg_index := self.GetMgIndex(mg_id)
	pe_index := self.mgs[mg_index].GetPeIndex(pe_id)
	self.AddData(mg_index, pe_index, data)
}

func (self *Device) Clone() *Device {
	device := &Device{id: self.id, weight: self.weight, total: self.total}
	device.mgs = make([]*MG, 0)
	for _, v := range self.mgs {
		device.mgs = append(device.mgs, v.Clone())
	}
	self.mg_bucket.Clone(&device.mg_bucket)
	return device
}

func (self *Device) SetMgStandard(mg_index, standard uint32) {
	self.mgs[mg_index].standard = standard
}

func (self *Device) SetMgWeight(mg_index, weight uint32) {
	old_weight := self.mgs[mg_index].weight
	if weight >= old_weight {
		self.weight += weight - old_weight
	} else {
		self.weight -= old_weight - weight
	}

	self.mgs[mg_index].weight = weight
}

func (self *Device) SetPeStandard(mg_index, pe_index, standard uint32) {
	self.mgs[mg_index].pes[pe_index].standard = standard
}

func (self *Device) SetPeWeight(mg_index, pe_index, weight uint32) {
	old_weight := self.mgs[mg_index].pes[pe_index].weight
	if weight >= old_weight {
		self.weight += weight - old_weight
		self.mgs[mg_index].weight += weight - old_weight
	} else {
		self.weight -= old_weight - weight
		self.mgs[mg_index].weight -= old_weight - weight
	}

	self.mgs[mg_index].SetPeWeight(pe_index, weight)
}

func (self *Device) PrintCount() string {
	str := fmt.Sprintf("Device[%d]: total = %d\n", self.id, self.total)
	for _, mg := range self.mgs {
		str += mg.PrintCount()
	}
	return str
}

func (self *Device) PrintWeight() string {
	str := fmt.Sprintf("Device[%d]: weight = %d\n", self.id, self.weight)
	for _, mg := range self.mgs {
		str += mg.PrintWeight()
	}
	return str
}

func (self *Device) PrintData() string {
	str := fmt.Sprintf("Device[%d]: total = %d\n", self.id, self.total)
	for _, mg := range self.mgs {
		str += mg.PrintData()
	}
	return str
}

func (self *Device) PrintStat() string {
	str := fmt.Sprintf("Device[%d]: total = %d\n", self.id, self.total)
	for _, mg := range self.mgs {
		str += mg.PrintStat()
	}
	str += self.stat.String()
	return str
}

func (self *Device) PrintMigrate() string {
	str := fmt.Sprintf("Device[%d]:\n", self.id)

	for _, v := range self.mgs {
		str += v.PrintMigrate()
	}
	return str
}

func (self *Device) String() string {
	str := fmt.Sprintf("Device[%d]: weight = %d, total = %d\n", self.id, self.weight, self.total)
	for _, mg := range self.mgs {
		str += mg.String()
	}
	return str
}

func (self *Device) StatDistribution() string {
	str := fmt.Sprintf("Device[%d]: weight = %d, total = %d\n", self.id, self.weight, self.total)
	for _, mg := range self.mgs {
		str += mg.String()
	}
	return str
}

func (self *Device) SetStandard(total uint32) {
	mg_standard := total / self.Size()
	for _, mg := range self.mgs {
		mg.SetStandard(mg_standard)
	}
}

func (self *Device) Migrate(from_mg_id, from_pe_id, to_mg_id, to_pe_id, data uint32) {
	from_mg_index := self.GetMgIndex(from_mg_id)
	to_mg_index := self.GetMgIndex(to_mg_id)
	from_pe_index := self.mgs[from_mg_index].GetPeIndex(from_pe_id)
	to_pe_index := self.mgs[to_mg_index].GetPeIndex(to_pe_id)

	self.mgs[from_mg_index].MigrateOutData(from_pe_index, data)
	self.mgs[to_mg_index].MigrateInData(to_pe_index, data)
}

func (self *Device) FindMgById(mg_id uint32) bool {
	for _, v := range self.mgs {
		if v.id == mg_id {
			return true
		}
	}
	return false
}

func (self *Device) ScaleOutMg(mg_id, pe_num, pe_weight uint32) *Device {
	if self.FindMgById(mg_id) {
		fmt.Println("ScaleOutMg error: mg_id exist, need not scale out")
		return self
	}

	device := self.Clone()
	mg := NewMG(mg_id, pe_num, pe_weight)
	device.AddMg(mg)

	for _, v := range device.mgs {
		if v.id != mg_id {
			v.ScaleOutMg(device)
		}
	}

	return device
}

func (self *Device) ScaleInMg(mg_id uint32) *Device {
	if !self.FindMgById(mg_id) {
		fmt.Println("ScaleInMg error: mg_id not exist, need not scale in")
		return self
	}

	device := self.Clone()

	mg_index := device.GetMgIndex(mg_id)
	device.mg_bucket.DelItem(mg_index)

	device.mgs[mg_index].ScaleInMg(device)
	device.DelMg(mg_id)

	return device
}

func NewRands(num uint32) map[uint32]uint32 {
	rands := make(map[uint32]uint32)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for uint32(len(rands)) < num {
		x := r.Uint32()
		//fmt.Println("x =", x)
		if _, ok := rands[x]; ok {
			continue
		}
		rands[x] = x
	}
	return rands
}

func main() {

	rands := NewRands(4000000)
	fmt.Println(len(rands))

	sbc := NewDevice(2, 20, 4)

	fmt.Printf("------------------------------------------------\n")
	fmt.Printf("power on\n")
	fmt.Printf("------------------------------------------------\n")

	start_time := time.Now()
	for key, _ := range rands {
		mg_id, pe_id := sbc.Select(key)
		sbc.AddDataById(mg_id, pe_id, key)
	}
	elapsed := time.Since(start_time)

	fmt.Printf("%s", sbc.PrintCount())
	fmt.Printf("%s", sbc.PrintMigrate())
	fmt.Printf("use time: %v\n", elapsed)

	//*
	fmt.Printf("------------------------------------------------\n")
	fmt.Printf("Scale out: add MG[100], PE_Num = 20, PE weight=4\n")
	fmt.Printf("------------------------------------------------\n")

	start_time = time.Now()
	new_sbc := sbc.ScaleOutMg(100, 20, 4)
	elapsed = time.Since(start_time)

	fmt.Printf("%s", new_sbc.PrintCount())
	fmt.Printf("%s", new_sbc.PrintMigrate())
	fmt.Printf("use time: %v\n", elapsed)

	fmt.Printf("------------------------------------------------\n")
	fmt.Printf("Scale out: add MG[101], PE_Num = 20, PE weight=4\n")
	fmt.Printf("------------------------------------------------\n")

	start_time = time.Now()
	new_sbc = new_sbc.ScaleOutMg(101, 20, 4)
	elapsed = time.Since(start_time)

	fmt.Printf("%s", new_sbc.PrintCount())
	fmt.Printf("%s", new_sbc.PrintMigrate())
	fmt.Printf("use time: %v\n", elapsed)

	fmt.Printf("------------------------------------------------\n")
	fmt.Printf("Scale in: del MG[101]\n")
	fmt.Printf("------------------------------------------------\n")

	start_time = time.Now()
	new_sbc = new_sbc.ScaleInMg(101)
	elapsed = time.Since(start_time)

	fmt.Printf("%s", new_sbc.PrintCount())
	fmt.Printf("%s", new_sbc.PrintMigrate())
	fmt.Printf("use time: %v\n", elapsed)
	//*/
}
