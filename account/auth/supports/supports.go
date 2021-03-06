package supports

import (
	"../../../managers/dbmanager"
	"../../../params/authparams"
	"../../../utils/randworker"
	"../../aescryption"
	"database/sql"
	"errors"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"time"
)

func QueryProcess(uid int64, mp *map[string]string) (err error) {
	// get connection
	db, err := dbmanager.DialPG()
	if err != nil {
		return
	}
	defer db.Close()
	// prepare
	stmt, err := db.Prepare("SELECT username, password, phone, stuid, wxopenid FROM account WHERE uid=$1")
	if err != nil {
		return
	}
	// act
	res, err := stmt.Query(uid)
	if err != nil {
		return
	}
	if res.Next() {
		var nun, npw, nph, nst, nwx sql.NullString
		err = res.Scan(&nun, &npw, &nph, &nst, &nwx)
		// must allocate memory for mp
		*mp = make(map[string]string)
		m := *mp
		m["username"] = nun.String
		//m["password"] = npw.String
		if npw.String != "" {
			m["password"] = "set"
		} else {
			m["password"] = ""
		}
		m["phone"] = nph.String
		m["stuid"] = nst.String
		m["wxopenid"] = nwx.String
		log.WithFields(log.Fields{
			"action":   "queryAuth",
			"error":    err,
			"uid":      uid,
			"username": nun,
			"password": npw,
			"phone":    nph,
			"stuid":    nst,
			"wxopenid": nwx,
		}).Info()
		return
	} else {
		err = errors.New("uid not found")
		return
	}
}

func QueryAuth(id string, account string) (uid int64, password string, pt string, pl int64, nk string, err error) {
	// get connection
	db, err := dbmanager.DialPG()
	if err != nil {
		return
	}
	defer db.Close()
	// prepare
	stmt, err := db.Prepare("SELECT uid, password, privilegetype, privilegelevel, nickname FROM account WHERE " + id + "=$1")
	if err != nil {
		return
	}
	// act
	res, err := stmt.Query(account)
	if err != nil {
		return
	}
	// deal res
	if res.Next() {
		var nuid sql.NullInt64
		var npassword, npt, nnk sql.NullString
		var npl sql.NullInt64
		err = res.Scan(&nuid, &npassword, &npt, &npl, &nnk)
		uid = nuid.Int64
		password = npassword.String
		pt = npt.String
		pl = npl.Int64
		nk = nnk.String
		log.WithFields(log.Fields{
			"action":   "queryAuth",
			"error":    err,
			"uid":      uid,
			"password": password,
			"pt":       pt,
			"pl":       pl,
			"nk":       nk,
		}).Info()
		return
	} else {
		err = errors.New("uid not found")
		return
	}
}

func CreateAccount(secret *authparams.Params) (err error) {
	// get connection
	db, err := dbmanager.DialPG()
	if err != nil {
		return
	}
	defer db.Close()
	var res sql.Result
	var stmt *sql.Stmt
	if secret.AccountType == "phone" && secret.CodeType == "code" {
		// phone & code Create type
		stmt, err = db.Prepare("INSERT INTO account(registerprocess, registertype, nickname, phone) VALUES ('auth', 'phone', $1, $2);")
		if err != nil {
			return
		}
		res, err = stmt.Exec(secret.Account, secret.Account)
		if err != nil {
			return
		}
	} else if secret.AccountType == "username" && secret.CodeType == "password" {
		// username & password Create type
		stmt, err = db.Prepare("INSERT INTO account(registerprocess, registertype, nickname, username, password) VALUES ('auth', 'password', $1, $2, $3)")
		if err != nil {
			return
		}
		res, err = stmt.Exec(secret.Account, secret.Account, secret.Code)
		if err != nil {
			return
		}
	} else if secret.AccountType == "wxopenid" {
		// wxOpenid Create type
		stmt, err = db.Prepare("INSERT INTO account(registerprocess, registertype, nickname, wxopenid) VALUES ('database', 'wxopenid', $1, $1);")
		if err != nil {
			return
		}
		res, err = stmt.Exec(secret.Account, secret.Account)
		if err != nil {
			return
		}
	} else {
		err = errors.New("wrong type")
		return
	}
	log.WithFields(log.Fields{
		"action":      "CreateAccount",
		"error":       err,
		"account":     secret.Account,
		"accountType": secret.AccountType,
		"code":        secret.Code,
		"codeType":    secret.CodeType,
	}).Info()
	row, err := res.RowsAffected()
	if err != nil {
		return
	}
	if row != 1 {
		err = errors.New("error rows effect")
	}
	return
}

func ChangeAccount(uid int64, id string, target string) (err error) {
	// get connection
	db, err := dbmanager.DialPG()
	if err != nil {
		return
	}
	defer db.Close()
	// prepare
	stmt, err := db.Prepare("UPDATE account SET " + id + "=$1 WHERE uid=$2;")
	if err != nil {
		return
	}
	res, err := stmt.Exec(target, uid)
	if err != nil {
		return
	}
	row, err := res.RowsAffected()
	if row != 1 {
		err = errors.New("error rows effect")
	}
	return
}

func DecodeToken(token string) (uid int64, pt string, pl int64, nk string, t time.Time, err error) {
	var message string
	message, err = aescryption.Decrypt(token)

	set := strings.Split(message, "&")
	if len(set) < 5 {
		err = errors.New("token message wrong")
		return
	}
	uid, err = strconv.ParseInt(set[0], 10, 64)
	pt = set[1]
	pl, err = strconv.ParseInt(set[2], 10, 64)
	nk = set[3]
	tt := set[4]
	t, err = time.Parse(time.RFC3339, tt)
	log.WithFields(log.Fields{
		"action": "decodeToken",
		"error":  err,
		"uid":    uid,
		"pt":     pt,
		"pl":     pl,
		"nk":     nk,
		"time":   t,
		"token":  token,
	}).Info()
	return
}

func MakeToken(uid int64, pt string, pl int64, nk string) (token string, err error) {
	limit := time.Now().Add(14 * 24 * time.Hour)
	message := strconv.FormatInt(uid, 10) + "&" + pt + "&" + strconv.FormatInt(pl, 10) + "&" + nk + "&" + limit.Format(time.RFC3339)
	token, err = aescryption.Encrypt(message)
	log.WithFields(log.Fields{
		"action": "makeToken",
		"error":  err,
		"uid":    uid,
		"pt":     pt,
		"pl":     pl,
		"nk":     nk,
		"time":   limit,
		"token":  token,
	}).Info()
	return
}

func MakeSecret(uid int64) (token string, err error) {
	token = randworker.GetAlnumString(32)
	_, err = dbmanager.SetCacheWithPX("secret&uid="+strconv.FormatInt(uid, 10), token, 300000)
	log.WithFields(log.Fields{
		"action": "makeSecret",
		"error":  err,
		"uid":    uid,
		"token":  token,
	}).Info()
	return
}

func CheckSecret(uid int64, token string) (err error) {
	res, err := dbmanager.GetCache("secret&uid=" + strconv.FormatInt(uid, 10))
	if err != nil {
		return
	}
	if res != token {
		err = errors.New("secret incorrect")
	} else {
		_, err = dbmanager.DelCache("secret&uid=" + strconv.FormatInt(uid, 10))
	}
	return
}

func SendCodeWithPhone(phone string) (err error) {
	str := "auth&phone=" + phone
	_, err = dbmanager.SetCacheWithPX(str, randworker.GetNumbersString(4), 300000)
	return
}

func CheckCodeWithPhone(phone string, code string) (err error) {
	str := "auth&phone=" + phone
	c, err := dbmanager.GetCache(str)
	if err != nil {
		return
	}
	if code == "" || c != code {
		err = errors.New("incorrect")
	} else {
		_, err = dbmanager.DelCache(str)
	}
	return
}

func SendCodeWithUid(uid int64) (err error) {
	str := "auth&uid=" + strconv.FormatInt(uid, 10)
	_, err = dbmanager.SetCacheWithPX(str, randworker.GetNumbersString(4), 300000)
	return
}

func CheckCodeWithUid(uid int64, code string) (err error) {
	str := "auth&uid=" + strconv.FormatInt(uid, 10)
	c, err := dbmanager.GetCache(str)
	if err != nil {
		return
	}
	if code == "" || c != code {
		err = errors.New("incorrect")
	} else {
		_, err = dbmanager.DelCache(str)
	}
	return
}
