package loggable

import (
	"fmt"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"testing"
)

var db *gorm.DB

type Person struct {
	gorm.Model
	LoggableModel
	Name   string
	UserID uint
}

func (p Person) Meta() interface{} {
	return struct {
		UserID uint
	}{
		UserID: p.UserID,
	}
}

func TestMain(m *testing.M) {
	database, err := gorm.Open(
		"mysql",
		fmt.Sprintf(
			"%v:%v@tcp(%v:%v)/%v?charset=utf8&parseTime=True&loc=Local",
			"root",
			"sesame",
			"localhost",
			3306,
			"loggable",
		),
	)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	database = database.LogMode(true)

	_, err = Register(database, ComputeDiff())
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	err = database.AutoMigrate(Person{}).Error
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	db = database
	m.Run()
}

func TestTryModel(t *testing.T) {
	newmodel := Person{
		Name:  "Pat",
		UserID: 20,
	}
	err := db.Create(&newmodel).Error
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(newmodel.ID)

	newmodel.Name = "John"
	err = db.Model(Person{}).Save(&newmodel).Error
	if err != nil {
		t.Fatal(err)
	}
}
