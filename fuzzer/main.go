package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/BattlesnakeOfficial/rules"
	"github.com/BattlesnakeOfficial/rules/client"
	"github.com/BattlesnakeOfficial/rules/maps"
)

type SimulateRequest struct {
	Game  client.SnakeRequest `json:"game"`
	Moves []rules.SnakeMove   `json:"moves"`
}

func convertBoardToState(board client.Board) *rules.BoardState {
	return rules.NewBoardState(board.Width, board.Height).
		WithFood(convertCoordsToPoints(board.Food)).
		WithHazards(convertCoordsToPoints(board.Hazards)).
		WithSnakes(convertSnakes(board.Snakes))
}

func convertCoordsToPoints(coords []client.Coord) []rules.Point {
	var points []rules.Point
	for _, coord := range coords {
		points = append(points, rules.Point{X: coord.X, Y: coord.Y})
	}
	return points
}

func convertSnakes(orig []client.Snake) []rules.Snake {
	var snakes []rules.Snake
	for _, snake := range orig {
		snakes = append(snakes, convertSnake(snake))
	}
	return snakes
}

func convertSnake(orig client.Snake) rules.Snake {
	return rules.Snake{
		ID:     orig.ID,
		Health: orig.Health,
		Body:   convertCoordsToPoints(orig.Body),
	}
}

func createNextBoardState(boardState *rules.BoardState, moves []rules.SnakeMove) (*rules.BoardState, error) {
	ruleset := rules.NewRulesetBuilder().NamedRuleset("standard")

	// Load game map
	gameMapName := "standard"
	gameMap, err := maps.GetMap(gameMapName)
	if err != nil {
		return nil, fmt.Errorf("Failed to load game map %#v: %v", gameMapName, err)
	}

	// apply PreUpdateBoard before making requests to snakes
	boardState, pre_update_err := maps.PreUpdateBoard(gameMap, boardState, ruleset.Settings())
	if pre_update_err != nil {
		return boardState, fmt.Errorf("Error pre-updating board with game map: %w", pre_update_err)
	}

	_, boardState, execute_err := ruleset.Execute(boardState, moves)
	if execute_err != nil {
		return boardState, fmt.Errorf("Error updating board state from ruleset: %w", execute_err)
	}

	// apply PostUpdateBoard after ruleset operates on snake moves
	boardState, post_update_err := maps.PostUpdateBoard(gameMap, boardState, ruleset.Settings())
	if post_update_err != nil {
		return boardState, fmt.Errorf("Error post-updating board with game map: %w", post_update_err)
	}

	boardState.Turn += 1

	return boardState, nil
}

func convertStateToBoard(boardState *rules.BoardState) client.Board {
	return client.Board{
		Height:  boardState.Height,
		Width:   boardState.Width,
		Food:    client.CoordFromPointArray(boardState.Food),
		Hazards: client.CoordFromPointArray(boardState.Hazards),
		Snakes:  convertRulesSnakes(boardState.Snakes),
	}
}

func convertRulesSnake(snake rules.Snake) client.Snake {
	latencyMS := 0
	return client.Snake{
		ID:             snake.ID,
		Name:           "fuzzer",
		Health:         snake.Health,
		Body:           client.CoordFromPointArray(snake.Body),
		Latency:        fmt.Sprint(latencyMS),
		Head:           client.CoordFromPoint(snake.Body[0]),
		Length:         int(len(snake.Body)),
		Shout:          "",
		Customizations: client.Customizations{},
	}
}

func convertRulesSnakes(snakes []rules.Snake) []client.Snake {
	a := make([]client.Snake, 0)
	for _, snake := range snakes {
		if snake.EliminatedCause == rules.NotEliminated {
			a = append(a, convertRulesSnake(snake))
		}
	}
	return a
}

func hello(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Fatal(err)
	}
	bodyString := string(bodyBytes)

	fmt.Println("Request received", bodyString)

	var req SimulateRequest
	json_err := json.Unmarshal(bodyBytes, &req)
	if json_err != nil {
		http.Error(w, json_err.Error(), http.StatusBadRequest)
		return
	}

	boardState := convertBoardToState(req.Game.Board)

	newState, simulateErr := createNextBoardState(boardState, req.Moves)
	if simulateErr != nil {
		http.Error(w, simulateErr.Error(), http.StatusBadRequest)
		return
	}

	newBoard := convertStateToBoard(newState)

	json.NewEncoder(w).Encode(newBoard)
}

func main() {
	http.HandleFunc("/simulate", hello)

	http.ListenAndServe(":8090", nil)
}
