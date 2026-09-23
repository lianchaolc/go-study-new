package main

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"time"

	_ "modernc.org/sqlite" // 纯Go sqlite驱动，无CGO
)

// Record 记账记录
type Record struct {
	ID         int
	CreateTime string
	Type       int     //1=收入，2=支出
	Category   string  //分类
	Amount     float64 //金额
	Remark     string  //备注
}

var db *sql.DB

const dbFile = "account.db" // 数据库文件，生成在项目根目录

// 初始化数据库，自动建表
func initDB() error {
	var err error
	// 注意：驱动名是 sqlite，不是 sqlite3
	db, err = sql.Open("sqlite", dbFile)
	if err != nil {
		return err
	}
	// 建表语句，不存在就创建
	sqlCreate := `
	CREATE TABLE IF NOT EXISTS records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		create_time DATETIME NOT NULL,
		type INTEGER NOT NULL,
		category TEXT NOT NULL,
		amount REAL NOT NULL,
		remark TEXT
	);`
	// `db.Exec` 用来执行**没有返回行**的 SQL：`CREATE TABLE`建表、INSERT、UPDATE、DELETE。

	// >
	// > ✅ 建表语句 `sqlCreate` 正好适合用 Exec；查询 SELECT 要用`db.Query`/`db.QueryRow`，**不能用 Exec**。

	// ## 2. 两个返回值

	// 1. **第一个返回值：`sql.Result`**
	// 里面两个方法：

	// >
	// > 👉 **下划线 `_` = 空白标识符**：代表**我不需要这个返回值，直接丢弃**。
	// > 建表的时候，我们不需要获取新增 ID、影响行数，所以直接用`_`丢掉。
	//    - `LastInsertId()`：拿到自增 ID（插入新记录才有用）
	//    - `RowsAffected()`：拿到本次 SQL 影响了多少行（更新 / 删除才有用）
	// 2. **第二个返回值：`err error`**
	// 最重要！用来判断 SQL 执行**有没有报错**。
	//    - `err == nil`：执行成功
	//    - `err != nil`：失败（表已存在、语法错误、数据库连接失败等）
	_, err = db.Exec(sqlCreate)
	if err != nil {
		return err
	}
	return nil
}

func printMenu() {
	fmt.Println("\n===== SQLite记账本 =====")
	fmt.Println("1. 添加收支记录")
	fmt.Println("2. 查询全部记录")
	fmt.Println("3. 按分类查询")
	fmt.Println("4. 收支统计汇总")
	fmt.Println("5. 删除记录")
	fmt.Println("6. 修改记录")
	fmt.Println("0. 退出")
	fmt.Print("请输入选项：")
}

// 添加记录
func addRecord() {
	var t int
	fmt.Print("1=收入，2=支出：")
	fmt.Scan(&t)
	if t != 1 && t != 2 {
		fmt.Println("类型错误！")
		return
	}
	var category string
	fmt.Print("分类(工资/餐饮/交通等)：")
	fmt.Scan(&category)
	var amount float64
	fmt.Print("金额：")
	fmt.Scan(&amount)
	if amount <= 0 {
		fmt.Println("金额必须大于0")
		return
	}
	var remark string
	fmt.Print("备注：")
	fmt.Scan(&remark)
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := db.Exec(`INSERT INTO records(create_time,type,category,amount,remark) VALUES(?,?,?,?,?)`,
		now, t, category, amount, remark)
	if err != nil {
		fmt.Println("添加失败：", err)
		return
	}
	fmt.Println("✅ 添加成功")
}

// 查询全部
func showAll() {
	rows, err := db.Query(`SELECT id,create_time,type,category,amount,remark FROM records ORDER BY create_time DESC`)
	if err != nil {
		fmt.Println("查询失败：", err)
		return
	}
	defer rows.Close()

	fmt.Printf("\n%4s | %19s | %4s | %8s | %10s | %s\n",
		"ID", "时间", "类型", "分类", "金额", "备注")
	for rows.Next() {
		var r Record
		err := rows.Scan(&r.ID, &r.CreateTime, &r.Type, &r.Category, &r.Amount, &r.Remark)
		if err != nil {
			continue
		}
		typStr := "支出"
		if r.Type == 1 {
			typStr = "收入"
		}
		fmt.Printf("%4d | %19s | %4s | %8s | %10.2f | %s\n",
			r.ID, r.CreateTime, typStr, r.Category, r.Amount, r.Remark)
	}
}

// 按分类查询
func queryByCategory() {
	var cat string
	fmt.Print("输入要查询的分类：")
	fmt.Scan(&cat)
	rows, err := db.Query(`SELECT id,create_time,type,category,amount,remark FROM records WHERE category=? ORDER BY create_time DESC`, cat)
	if err != nil {
		fmt.Println("查询失败：", err)
		return
	}
	defer rows.Close()
	fmt.Printf("\n【分类：%s】\n", cat)
	for rows.Next() {
		var r Record
		err := rows.Scan(&r.ID, &r.CreateTime, &r.Type, &r.Category, &r.Amount, &r.Remark)
		if err != nil {
			continue
		}
		typStr := "支出"
		if r.Type == 1 {
			typStr = "收入"
		}
		fmt.Printf("%4d | %19s | %4s | %8s | %10.2f | %s\n",
			r.ID, r.CreateTime, typStr, r.Category, r.Amount, r.Remark)
	}
}

// 统计收支
func stat() {
	var income, expend float64
	_ = db.QueryRow(`SELECT IFNULL(SUM(amount),0) FROM records WHERE type=1`).Scan(&income)
	_ = db.QueryRow(`SELECT IFNULL(SUM(amount),0) FROM records WHERE type=2`).Scan(&expend)
	balance := income - expend
	fmt.Println("\n==== 收支统计 ====")
	fmt.Printf("总收入：%.2f\n", income)
	fmt.Printf("总支出：%.2f\n", expend)
	fmt.Printf("结余：%.2f\n", balance)
}

// 删除记录
func delRecord() {
	var idStr string
	fmt.Print("输入要删除记录ID：")
	fmt.Scan(&idStr)
	// `strconv.Atoi` = **string to int**，把字符串转成整数
	id, err := strconv.Atoi(idStr)
	if err != nil {
		fmt.Println("ID非法")
		return
	}
	_, err = db.Exec(`DELETE FROM records WHERE id=?`, id)
	if err != nil {
		fmt.Println("删除失败：", err)
		return
	}
	fmt.Println("✅ 删除成功")
}

// 修改记录
func editRecord() {
	var idStr string
	fmt.Print("输入要修改记录ID：")
	fmt.Scan(&idStr)
	id, err := strconv.Atoi(idStr)
	if err != nil {
		fmt.Println("ID非法")
		return
	}
	var t int
	fmt.Print("1=收入，2=支出：")
	fmt.Scan(&t)
	var category string
	fmt.Print("新分类：")
	fmt.Scan(&category)
	var amount float64
	fmt.Print("新金额：")
	fmt.Scan(&amount)
	var remark string
	fmt.Print("新备注：")
	fmt.Scan(&remark)
	_, err = db.Exec(`UPDATE records SET type=?,category=?,amount=?,remark=? WHERE id=?`, t, category, amount, remark, id)
	if err != nil {
		fmt.Println("修改失败：", err)
		return
	}
	fmt.Println("✅ 修改成功")
}

func main() {
	err := initDB()
	if err != nil {
		fmt.Printf("数据库初始化失败：%v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	fmt.Println("SQLite数据库打开成功，文件：", dbFile)

	for {
		printMenu()
		var opt string
		fmt.Scan(&opt)
		switch opt {
		case "1":
			addRecord()
		case "2":
			showAll()
		case "3":
			queryByCategory()
		case "4":
			stat()
		case "5":
			delRecord()
		case "6":
			editRecord()
		case "0":
			fmt.Println("程序退出")
			return
		default:
			fmt.Println("输入选项错误！")
		}
	}
}
