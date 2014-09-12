//Utility Functions
package main

import (
	"bufio"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"strconv"
)

//=============================================
//===============TYPES AND CONSTS==============
//=============================================
type roundName int
type Deck map[string]string
type state int
type money uint64
type guid string

const SEED int64 = 0 // seed for deal
var UNSHUFFLED = generateCardNames()

const (
	fold int = iota
	bet
	call
)
const (
	active state = iota
	folded
	called
)
const BUY_IN money = 500

type Player struct {
	state  state
	guid   guid
	wealth money
}

func (g *Game) deal(seed int64) {
	g.deck = make(Deck, 52)
	numPlayers := len(g.table.players)
	rand_ints := rand.New(rand.NewSource(seed)).Perm(52)
	for i := 0; i < numPlayers; i++ {
		card1, card2 := UNSHUFFLED[rand_ints[i*2]], UNSHUFFLED[rand_ints[i*2+1]]
		g.deck[card1] = string(g.table.players[i].guid)
		g.deck[card2] = string(g.table.players[i].guid)
	}
	n := numPlayers * 2
	g.deck[UNSHUFFLED[rand_ints[n+0]]] = "FLOP"
	g.deck[UNSHUFFLED[rand_ints[n+1]]] = "FLOP"
	g.deck[UNSHUFFLED[rand_ints[n+2]]] = "FLOP"
	g.deck[UNSHUFFLED[rand_ints[n+3]]] = "TURN"
	g.deck[UNSHUFFLED[rand_ints[n+4]]] = "RIVER"
}

//==========================================
//===============GAME CLASS=================
//==========================================
type Game struct {
	table      *Table
	pot        *Pot
	gameID     guid
	deck       Deck
	round      uint
	smallBlind money
	controller *controller
}

func newPot() *Pot {
	pot := new(Pot)
	pot.bets = make([]Bet, 0)
	return pot
}

func (g *Game) run() {
	//Testing Stuff
	defer gamePrinter(g)
	reader := bufio.NewReader(os.Stdin)
	//----

	for {
		println(">>")
		_, _ = reader.ReadString('\n')
		g.addWaitingPlayersToGame()
		if len(g.table.players) < 2 {
			continue //Need 2 players to start a hand
		}
		g.pot = newPot()
		g.removeBrokePlayers()
		g.betBlinds()
		g.deal(SEED)
		g.round = 0
		for i := 0; g.notSettled() && i < 4; i++ {
			println(">")
			_, _ = reader.ReadString('\n')
			gamePrinter(g)
			g.Bets()
			g.table.ResetRound()
			g.pot.newRound()
			g.round++
		}
		g.resolveBets()
		g.table.AdvanceButton()
		g.table.ResetHand()
	}
}

//Accepts a set of cards, represented as 2char strings and returns
// all nChooseK combinations of those cards
func nChooseK(allCards []string, k int) (allHands []Hand) {
	if k == 0 {
		return make([]Hand, 1)
	}

	for i := 0; i < len(allCards)-k+1; i++ {
		combinations := nChooseK(allCards[i+1:], k-1)
		for _, single_combination := range combinations {
			single_combination = append(single_combination, allCards[i])
			allHands = append(allHands, single_combination)
		}
	}

	return allHands
}

func (g *Game) generateAllHands(p *Player) (allHands []Hand) {
	allCards := make([]string, 0)
	for card, location := range g.deck {
		if location == string(p.guid) || location == "FLOP" || location == "TURN" || location == "RIVER" {
			allCards = append(allCards, card)
		}
	}
	if len(allCards) != 7 {
		panicMsg := fmt.Sprintf("Should have 7 cards. Have %v", allCards)
		panic(panicMsg)
	}
	return nChooseK(allCards, 5)
}

func (g *Game) getPlayerBestHand(p *Player) Hand {
	allHands := g.generateAllHands(p)
	bestHands := findWinningHands(allHands)
	if len(bestHands) < 1 {
		panicMsg := fmt.Sprintf("No best hand found for player %v, in hands: %v", p.guid, allHands)
		panic(panicMsg)
	}
	return bestHands[0]
}

func (g *Game) getAllPlayersBestHands() (bestHands map[guid]Hand) {
	bestHands = make(map[guid]Hand)
	for i, p := range g.table.players {
		fmt.Println("counting players; i = ", i)
		bestHand := g.getPlayerBestHand(p)
		bestHands[p.guid] = bestHand
		//		The below code does not apply if we stop betting when all but one player has folded
		// 		if p.state == active {
		//			panic("Active players still exist in resolveBets")
		//		}
	}
	fmt.Println("inside getAllPlayersBestHands(); len(bestHands) == ", len(bestHands))
	return bestHands
}

func (g *Game) aggregatePots() (participants map[uint][]guid, moneys map[uint]money) {
	participants = make(map[uint][]guid)
	moneys = make(map[uint]money)
	for _, bet := range g.pot.bets {
		if _, ok := participants[bet.potNumber]; !ok {
			participants[bet.potNumber] = make([]guid, 0)
		}
		participants[bet.potNumber] = append(participants[bet.potNumber], bet.player)
		moneys[bet.potNumber] += bet.value
	}
	return participants, moneys
}

func (g *Game) getWinningPlayers(playersToBestHands map[guid]Hand, guids []guid) (winners []guid) {
	hands := make([]Hand, 0)
	for _, id := range guids {
		hands = append(hands, playersToBestHands[id])
		for _, c := range playersToBestHands[id] {
			println("..", c)
		}
		println("$$$", id)
	}
	if len(hands) != len(g.table.players) {
		panicMsg := fmt.Sprintf("Did not get a best hand for each player: len of hands = %v; len of g.table.players = %v\n", len(hands), len(g.table.players))
		panic(panicMsg)
	}
	winningHands := findWinningHands(hands)
	winners = make([]guid, 0)
	for _, id := range guids {
		for _, h := range winningHands {
			if areHandsEq(playersToBestHands[id], h) {
				var p = new(Player)
				for _, player := range g.table.players {
					if player.guid == id {
						p = player
					}
				}
				if p.state != folded {
					winners = append(winners, id)
				}
				break
			}
		}
	}
	return winners
}

func (g *Game) resolveBets() {
	playersToBestHands := g.getAllPlayersBestHands()
	participantsInPots, moneyInPots := g.aggregatePots()

	for potNumber, guids := range participantsInPots {
		winners := g.getWinningPlayers(playersToBestHands, guids)
		for _, p := range g.table.players {
			for _, id := range winners {
				if p.guid == id {
					p.wealth += moneyInPots[potNumber] / money(len(winners))
					if moneyInPots[potNumber]%money(len(winners)) > 0 {
						p.wealth++
						moneyInPots[potNumber]--
					}
				}
			}
		}
	}
}

//notSettled returns true if >1 player is not folded
func (g *Game) notSettled() bool {
	notFolded := 0
	for _, p := range g.table.players {
		if p.state != folded {
			notFolded++
		}
	}
	return notFolded > 1
}

func (g *Game) addWaitingPlayersToGame() {
	numPlayersNeeded := (10 - len(g.table.players))
	newPlayers := g.controller.getNewPlayers(g, numPlayersNeeded)
	for _, p := range newPlayers {
		err := g.table.addPlayer(p.guid)
		if err != nil {
			panic(err)
		}
	}
}

func (g *Game) removeBrokePlayers() {
	for _, p := range g.table.players {
		if p.wealth == 0 {
			p.state = folded
			g.controller.removePlayerFromGame(g, p.guid)
		} else if p.wealth < 0 {
			panic("player has < 0 wealth!")
		}
	}
}

func (g *Game) betBlinds() {
	//Bet small blind
	player := g.table.Next()
	if player.wealth >= g.smallBlind {
		g.commitBet(player, g.smallBlind)
	} else {
		g.commitBet(player, player.wealth)
	}

	//Bet big blind
	player = g.table.Next()
	if player.wealth >= 2*g.smallBlind {
		g.commitBet(player, 2*g.smallBlind)
	} else {
		g.commitBet(player, player.wealth)
	}
}

//setBlinds sets the money amount for the blinds
// and rotates the "button"
func (g *Game) setBlinds() {
	g.smallBlind = 25
}

func (g *Game) betsNeeded() bool {
	numActives := 0
	numFolded := 0
	for _, p := range g.table.players {
		if p.state == active {
			numActives++
		} else if p.state == folded {
			numFolded++
		}
	}
	return (numActives >= 1) || !(len(g.table.players) == (1 + numFolded))
}

//Bets gets the bet from each player
func (g *Game) Bets() {
	for player := g.table.Next(); g.betsNeeded(); player = g.table.Next() {
		if player.state != active {
			continue
		}

		action, betAmount, err := g.controller.getPlayerBet(g, player.guid)
		//Illegit bets
		if err != nil {
			//Err occurs on connection timeout
			player.state = folded
			g.controller.removePlayerFromGame(g, player.guid)
			continue
		}
		if action == fold {
			player.state = folded
			continue
		}
		if g.betInvalid(player, betAmount) {
			g.controller.registerInvalidBet(g, player.guid, betAmount)
			player.state = folded
			continue
		}

		//Legit bets
		isRaising := (g.pot.totalPlayerBetThisRound(player.guid) + betAmount) > g.pot.totalToCall
		if isRaising {
			g.table.ResetRoundPlayerState()
		}
		g.commitBet(player, betAmount)
		player.state = called
	}

}

func (g *Game) betInvalid(player *Player, bet money) bool {
	return (bet > player.wealth) ||
		(bet < g.pot.minRaise) ||
		(bet < player.wealth && (g.pot.totalPlayerBetThisRound(player.guid)+bet) < g.pot.totalToCall)
}

func (g *Game) commitBet(player *Player, amount money) {
	if amount <= 0 {
		panic("trying to bet <= 0")
	}
	g.pot.receiveBet(player.guid, amount)
	player.wealth -= amount
}

//======================================
//===============POT AND BET============
//======================================
type Pot struct {
	minRaise    money
	totalToCall money
	potNumber   uint
	bets        []Bet
}

type Bet struct {
	potNumber uint
	player    guid
	value     money
}

//Resolves partial pots from previous round
// increments potNumber to a previously unused number
func (pot *Pot) newRound() {
	pot.condenseBets()
	pot.makeSidePots()
	pot.minRaise = 0
	pot.totalToCall = 0
}

func (pot *Pot) condenseBets() {
	playerBets := make(map[guid]money)
	betsCopy := make([]Bet, 0)
	for _, bet := range pot.bets {
		if bet.potNumber == pot.potNumber {
			playerBets[bet.player] += bet.value
		} else {
			betsCopy = append(betsCopy, bet)
		}
	}
	for k, v := range playerBets {
		betsCopy = append(betsCopy, Bet{potNumber: pot.potNumber, player: k, value: v})
	}

	pot.bets = betsCopy
}

func (pot *Pot) allBetsEqual(potNumber uint) bool {
	var prevBet money
	for _, bet := range pot.bets {
		if bet.potNumber == potNumber {
			prevBet = bet.value
			break
		}
	}
	for _, bet := range pot.bets {
		if bet.potNumber != potNumber {
			continue
		}
		if prevBet != bet.value {
			return false
		}
	}
	return true
}

func (pot *Pot) makeSidePots() {
	if pot.allBetsEqual(pot.potNumber) {
		return
	}
	pot.potNumber++ //Make a new pot

	minimum := money(math.MaxUint64)
	for _, b := range pot.bets {
		if b.value < minimum && b.potNumber == (pot.potNumber-1) {
			minimum = b.value
		}
	}
	for _, b := range pot.bets {
		if b.value > minimum && b.potNumber == (pot.potNumber-1) {
			excess := b.value - minimum
			b.value = minimum
			pot.bets = append(pot.bets, Bet{potNumber: pot.potNumber, player: b.player, value: excess})
		}
	}
	pot.makeSidePots() //Call again to split new side pot into more side pots if necessary
}

func (pot *Pot) receiveBet(guid guid, bet money) {
	newBet := Bet{potNumber: pot.potNumber, player: guid, value: bet}
	pot.bets = append(pot.bets, newBet)
	totalBet := pot.totalPlayerBetThisRound(guid)
	if totalBet > pot.totalToCall {
		pot.totalToCall = totalBet
	}
	raise := totalBet - pot.totalToCall
	if raise > pot.minRaise {
		pot.minRaise = 2 * raise
	}
}

func (pot *Pot) totalInPot() money {
	var sum money = 0
	for _, m := range pot.bets {
		sum += m.value
	}
	return sum
}

func (pot *Pot) totalPlayerBetThisRound(g guid) money {
	sum := money(0)
	for _, m := range pot.bets {
		if m.player == g && m.potNumber == pot.potNumber {
			sum += m.value
		}
	}
	return sum
}

//=====================================
//===============TABLE=================
//=====================================
type Table struct {
	players []*Player
	index   int
}

func (t *Table) addPlayer(p guid) (err error) {
	var newPlayer *Player = &Player{state: active, guid: p, wealth: 1000}
	if len(t.players) >= 10 {
		err = fmt.Errorf("Table full!")
		return err
	}

	t.players = append(t.players, newPlayer)
	return err

}

func (t *Table) AdvanceButton() {
	n := len(t.players)
	last := t.players[0]
	for i := 0; i < (len(t.players) - 1); i++ {
		t.players[i] = t.players[i+1]
	}
	t.players[n-1] = last
}

func (t *Table) ResetRoundPlayerState() {
	for _, p := range t.players {
		if p.state == called {
			p.state = active
		}
	}
}

func (t *Table) ResetHandPlayerState() {
	for _, p := range t.players {
		p.state = active
	}
}

func (t *Table) ResetRound() {
	t.index = 0
	t.ResetRoundPlayerState()
}

func (t *Table) ResetHand() {
	t.index = 0
	t.ResetHandPlayerState()
}

func (t *Table) Next() (p *Player) {
	p = t.players[t.index]
	t.index = (t.index + 1) % len(t.players)
	return p
}

func NewGame(gc *GameController) (g *Game) {
	g = new(Game)
	g.table = new(Table)
	g.table.players = make([]*Player, 0)
	g.pot = new(Pot)
	g.pot.bets = make([]Bet, 0)
	g.controller = NewController()
	g.smallBlind = 10
	return g
}

//===========================================
//===============HELPERS=====================
//===========================================
func createGuid() string {
	return s4() + s4() + "-" + s4() + "-" + s4() + "-" + s4() + "-" + s4() + s4() + s4()
}

func s4() string {
	s := ""
	for i := 0; i < 4; i++ {
		n := rand.Int63n(16)
		s += strconv.FormatInt(n, 16)
	}
	return s
}

func generateCardNames() (deck [52]string) {
	suits := []string{"S", "C", "D", "H"}
	ranks := []string{"2", "3", "4", "5", "6", "7", "8", "9", "T", "J", "Q", "K", "A"}
	i := 0
	for _, suit := range suits {
		for _, rank := range ranks {
			deck[i] = rank + suit
			i++
		}
	}
	return deck
}

func (t *Table) contains(id guid) bool {
	for _, player := range t.players {
		if player.guid == id {
			return true
		}
	}
	return false
}

func (t *Table) remove(id guid) {
	var index int
	for i, player := range t.players {
		if player.guid == id {
			index = i
		}
	}
	if index == len(t.players)-1 {
		t.players = t.players[:index]
	} else {
		t.players = append(t.players[:index], t.players[index+1:]...)
	}
}

func (d Deck) String() string {
	ordered := make([]string, 0)
	for card, location := range d {
		if len(location) > 5 {
			location = string(location[:5])
		}
		ordered = append(ordered, location+":"+card)
	}
	ordered = sort.StringSlice(ordered)
	s := "map[\n"
	for _, card := range ordered {
		s += "  "
		s += card
		s += "\n"
	}
	s += "\n"
	return s
}
