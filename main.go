package main

import (
    "database/sql"
    "fmt"
    "log"
    "os"
    "time"
    "net/http"
    "net/url"
    "encoding/base64"
    "encoding/json"
    "io/ioutil"
    "strings"
    _ "github.com/go-sql-driver/mysql"
    "github.com/aws/aws-lambda-go/lambda"
)

type SmartcarResponse struct {
    AccessToken    string `json:"access_token"`
    TokenType     string `json:"token_type"`
    ExpiresIn     int    `json:"expires_in"`
    RefreshToken  string `json:"refresh_token"`
}

func connect_db() (*sql.DB, error) {
    dbHost := os.Getenv("RDS_HOST")
    dbUser := os.Getenv("RDS_USERNAME")
    dbPassword := os.Getenv("RDS_PASSWORD")
    dbName := os.Getenv("RDS_DB_NAME")
    
    conString := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s", dbUser, dbPassword, dbHost, dbName)
    db, err := sql.Open("mysql", conString)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to database: %w", err)
    }
    
    if err := db.Ping(); err != nil {
        db.Close()
        return nil, fmt.Errorf("failed to ping database: %w", err)
    }
    
    return db, nil
}

func refresh_token(db *sql.DB, id string, currentRefreshToken string) error {
    clientID := os.Getenv("SMARTCAR_CLIENT_ID")
    clientSecret := os.Getenv("SMARTCAR_CLIENT_SECRET")
    
    auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", clientID, clientSecret)))
    
    data := url.Values{}
    data.Set("grant_type", "refresh_token")
    data.Set("refresh_token", currentRefreshToken)
    
    req, err := http.NewRequest("POST", "https://auth.smartcar.com/oauth/token", strings.NewReader(data.Encode()))
    if err != nil {
        return err
    }
    
    req.Header.Set("Authorization", "Basic "+auth)
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return err
    }
    
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("API error: %s", string(body))
    }
    
    var tokenResp SmartcarResponse
    if err := json.Unmarshal(body, &tokenResp); err != nil {
        return err
    }
    
    _, err = db.Exec(`
        UPDATE tokens 
        SET accessToken = ?, 
            refreshToken = ?,
            refreshTokenExpiryDate = ?,
            refreshAccessExpiryDate = ?
        WHERE driver_fk_id = ?`,
        tokenResp.AccessToken,
        tokenResp.RefreshToken,
        time.Now(),
        time.Now(),
        id)
    
    return err
}

func update_tokens(db *sql.DB) error {
    rows, err := db.Query(`
        SELECT driver_fk_id, refreshToken 
        FROM tokens 
        WHERE refreshTokenExpiryDate < DATE_ADD(NOW(), INTERVAL 15 DAY)`)
    if err != nil {
        return err
    }
    defer rows.Close()
    
    for rows.Next() {
        var id, refreshToken string
        if err := rows.Scan(&id, &refreshToken); err != nil {
            log.Printf("Error scanning row: %v", err)
            continue
        }
        
        if err := refresh_token(db, id, refreshToken); err != nil {
            log.Printf("Error refreshing token for ID %s: %v", id, err)
            continue
        }
        
        log.Printf("Successfully refreshed tokens for ID: %s", id)
    }
    
    return rows.Err()
}

func handler(){
    db, err := connect_db()
    if err != nil {
        log.Fatalf("Database connection error: %v", err)
    }
    defer db.Close()
    
    if err := update_tokens(db); err != nil {
        log.Fatalf("Error updating tokens: %v", err)
    }
}

func main() {
    lambda.Start(handler)
}
