(function() {
    angular.module('project', ['ngRoute'])

    .config(function($routeProvider) {
        $routeProvider
            .when('/', {
                controller: 'HomeControl',
                templateUrl: '/public/templates/Home.html',
            })
            .when('/login', {
                controller: 'LoginControl',
                templateUrl: '/public/templates/Login.html',
            })
			.when("/profile/:userID",{
				controller:"ProfileControl",
				templateUrl:"/public/templates/Profile.html"
			})
			.when("/gameCreation",{
				controller:"CreationControl",
				templateUrl:"/public/templates/GameCreation.html"
			})
			.when("/game/:roomId",{
				controller:"GameControl",
				templateUrl:"/public/templates/GameScreen.html"
			})
			.when("/settings",{
				controller:"SettingsControl",
				templateUrl:"/public/templates/Settings.html"
			})
            .otherwise({
                redirectTo: '/',
            });
    })

    .factory("user", function(){
        var user = {};
        return user;
    })
	
	.factory("chat", function($http) {
        var friends = [];
		var windowList=[];
        var socket = null;
        var $chatScope = null;
        var friendRequestHandler = null;
		var gameInviteHandler = null;

        function update(alsoUpdateStatus) {
            $http.get('/api/social/getFriendList')
                 .success(function(data, status, headers, config) {
                      if (data.Code == 0) {
                          friends.length = 0;
                          for (i = 0; i < data.Friends.length; i++) {
                              friends.push(data.Friends[i]);
                          }
                          
                          if (alsoUpdateStatus) {
                              socket.send(JSON.stringify({To: 0, Message: "GetOnlineStatus"}));
                          }   
                      }
					  // window list got reset, so it's going to refresh the DOM
					  // by default div's don't scroll to the bottom, so we need to send them back down to the end
					  function scrollWindows(){
						windowList.forEach(function(Id){
							var divName="#chatWindow_messageLog_"+Id;
							if($(divName)==undefined){
								console.log("didn't find the message log container");
								return;
							}
							$(divName).stop().animate({scrollTop: $(divName)[0].scrollHeight}, 'fast');
						});
					  }
					  // dom not ready yet, queue task
					  setTimeout(scrollWindows,0);
                 });
        }

        function openWSConnection() {
            var WS_URL = "ws://" + location.host + "/api/social/feed";
            socket = new WebSocket(WS_URL); // WebSocket object for social service
            socket.onopen = function() {
                socket.send(JSON.stringify({To: 0, Message: "GetOnlineStatus"}));
                console.log("Social feed stream established");
            };
            socket.onclose = function() {
                console.log("Social feed stream closed");
            };
            socket.onerror = function(e) {
                console.log("Social feed stream error: ", e);
            };
            socket.onmessage = function(e) {
                e = JSON.parse(e.data);
                if (e.Type == 1 || e.Type == 3 || e.Type == 2) { // status notification
                    for (i = 0; i < friends.length; i++) {
                        if (friends[i].Id == e.From) {
                            if (e.Type == 1) {
                                friends[i].Status = 0; // Online
                            } else if (e.Type == 3) {
                                friends[i].Status = 2; // Offline
                            } else if (e.Type == 2) {
                                friends[i].Status = 1; // Offline
                            }
                            $chatScope.$apply();
                        }
                    }
                } else if (e.Type == 0) { // new message
                    if (e.From == 0) { // system message
                        if (e.Content == 'Refresh') {
                            update(true);
                        }
                    } else {
                        $chatScope.receiveMessage(e.From, e.Content);
                        $chatScope.$apply();
                    }
                } else if (e.Type == 4) { // friend request
                    friendRequestHandler(e.From, e.Content);
                } else if (e.Type == 5) { // game invite
					gameInviteHandler(e.From, JSON.parse(e.Content));
				}
                console.log("Social feed message: ", e);
            };
        };

        function sendMessage(to, msg) { // send message to a friend, to must be int
            socket.send(JSON.stringify({To: to, Message: msg}));
        };

		return {'friends': friends, 
				'windowList':windowList,
                'update': update,
                'openWSConnection': openWSConnection,
                'setChatScope': function($s) {$chatScope = $s;},
                'sendMessage': sendMessage,
                'setFriendRequestHandler' : function(handler) {friendRequestHandler = handler;},
				'setGameInviteHandler': function(handler) {gameInviteHandler = handler;}
                };
	})

    .factory('game', function(user) {
        var SYSTEM = 0;
        var CHAT = 1;
        var JOIN = 2;
        var LEAVE = 3;
        var GAMEMESSAGE = 4;
        var REPORT = 5;
        var socket = null;
        var chatHandler = null;
        var joinHandler = null;
        var leaveHandler = null;
        var startHandler = null;
        var gameMessageHandler = null;
        var switchMasterHandler = null;
        var gameStarted = false;

        function openWSConnection(roomId, cb) {
            gameStarted = false;
            var WS_URL = "ws://" + location.host + "/api/game/socket/" + roomId;
            socket = new WebSocket(WS_URL); // WebSocket object for game service
            socket.onopen = function() {
                socket.send(JSON.stringify({Type: SYSTEM, From: user.Id, Content: "GetRoomUserList"}));
                console.log("Room socket established");
                cb();
            };
            socket.onclose = function() {
                console.log("Room socket closed");
            };
            socket.onerror = function(e) {
                console.log("Room socket error: ", e);
            };
            socket.onmessage = function(e) {
                e = JSON.parse(e.data);
                if (e.Type == SYSTEM) { // System notification 
                    if (e.Content == "StartGame") {
                        gameStarted = true;
                        startHandler();
                    } else if (e.Content == "SwitchMaster") {
                        switchMasterHandler(e.From);
                    }
                } else if (e.Type == CHAT) { // Chat
                    chatHandler(e.From, e.Content); // call the chat callback
                } else if (e.Type == JOIN) { // Join
                    joinHandler(e.From, e.Content);
                } else if (e.Type == LEAVE) { // Leave
                    leaveHandler(e.From, e.Content);
                } else if (e.Type == GAMEMESSAGE) {
                    gameMessageHandler(e.From, JSON.parse(e.Content));
                }
                console.log("Room socket message: ", e);
            };
        };

        function sendChatMessage(message) {
            socket.send(JSON.stringify({Type: CHAT, From: user.Id, Content: message}));
        };

        function closeWSConnection() {
            socket.close();
        };

        function startGame() {
            if (gameStarted) {
                return;
            }
            gameStarted = true;
            socket.send(JSON.stringify({Type: SYSTEM, From: user.Id, Content: 'StartGame'}));
        };

        function sendGameMessage(obj) {
            socket.send(JSON.stringify({Type: GAMEMESSAGE, From: user.Id, Content: JSON.stringify(obj)}));
        };

        function reportResult(type, winner) {
            if (!gameStarted) {
                return;
            }
            socket.send(JSON.stringify({Type: REPORT, From: user.Id, Content: JSON.stringify({'Type': type, 'Winner': winner})}));
        };

        return {
                'sendChatMessage': sendChatMessage,
                'setChatHandler': function(cb) {chatHandler = cb;},
                'setJoinHandler': function(cb) {joinHandler = cb;},
                'setLeaveHandler': function(cb) {leaveHandler = cb;},
                'openWSConnection': openWSConnection,
                'closeWSConnection': closeWSConnection,
                'startGame': startGame,
                'setStartHandler': function(cb) {startHandler = cb;},
                'setGameMessageHandler': function(cb) {gameMessageHandler = cb;},
                'setSwitchMasterHandler': function(cb) {switchMasterHandler = cb;},
                'sendGameMessage': sendGameMessage,
                'reportResult': reportResult
               };
    })

    .run(function($http, user, chat) {
            $http.get('/api/isloggedin')
                 .success(function(data, status, headers, config) {
                      if (data.Code == 0) {
                          user.Name = data.Name;
                          user.FirstName = data.FirstName;
                          user.LastName = data.LastName;
                          user.Id = data.Id;
						  // user.AvatarURL=... // TODO
                          chat.update();
                          chat.openWSConnection();
                      }
                 });
    })

    .controller('NavControl', function($scope, $location, $rootScope, user) {
        $scope.$location = $location;
        $scope.User = user;
        $scope.logout = function() {
            window.location.replace('/logout');
        };
    })

    .controller('InboxControl', function($scope, $rootScope, chat, $http, $location) {
        $rootScope.showInbox = false;
        $rootScope.newMessage = false;

        var FRIEND_REQUEST = 0;
        var GAME_INVITATION = 1;

        var messages = [];
        chat.setFriendRequestHandler(function(from, name) {
            messages.push({'Message': name + ' wants to add you as a friend. ', 'From': from, 'Type': FRIEND_REQUEST});
            $rootScope.newMessage = !$rootScope.showInbox;
            $rootScope.$apply();
        });
		chat.setGameInviteHandler(function(from, param){
			messages.push({'Message': param.Name + ' has invited you to play game "' + param.Game  + '".', 
                'From':from, 
                'Type': GAME_INVITATION,
                'RoomId': param.RoomId,
                'RoomPassword': param.RoomPassword});
            $rootScope.newMessage = !$rootScope.showInbox;
            $rootScope.$apply();
		});
		
        $scope.messages = messages;

        $scope.accept = function(index) {
            switch(messages[index].Type) {
				case FRIEND_REQUEST:
					$http.get('/api/social/acceptFriendRequest/' + messages[index].From)
						 .success(function(data, status, headers, config) {
							  if (data.Code == 0) {
								  chat.update(true);
							  } else if (data.Code == 1) {
								  alert("Invitation expired");
							  } 
						 });
				break;
				case GAME_INVITATION:
                     $rootScope.roomPassword = messages[index].RoomPassword; // ugly, should use service
                     $location.path('/game/' + messages[index].RoomId);
				break;
            }
            messages.splice(index, 1);
        };

        $scope.decline = function(index) {
            messages.splice(index, 1);
        };
    })
	
	.controller('SocialControl',function($scope, user, chat, $http){
        chat.setChatScope($scope);
		$scope.displayChatWindow=false;
		$scope.displayFriendWindow=true;
		$scope.addFriendTextBox = false;
		$scope.toggleSocial=function(n){
			if(n==undefined){n="fast";}
			$scope.displayFriendWindow=!$scope.displayFriendWindow;
			//if($scope.displayFriendWindow){
			$('#friendWindow').stop().animate({width: "toggle"}, n);
			if($scope.displayFriendWindow){
				$('#MainContentContainer').stop().animate({left: 300}, n);
			}
			else{
				$('#MainContentContainer').stop().animate({left: 0}, n);
			}
		}
		$scope.toggleSocial(0);
		
        $scope.friends = chat.friends;
		$scope.chatLogs=[];
		$scope.chatWindowList=chat.windowList;
        $scope.User = user;

        $scope.showCard = function(f, $event) {
            $($event.currentTarget).popover({
                'title': '<h5>' + f.FirstName + ' "' + f.Name  + '" ' + f.LastName + '</h5>',
                'content': f.Signature, 
                'container': 'body',
                'html': true,
                'trigger': 'hover',
            });
            $($event.currentTarget).popover('show');
        };

        $scope.closeChat = function(Id) {
			// remove any with a matching name
			for(var i=0;i<chat.windowList.length;i++){
				var id=chat.windowList[i];
				if(id==Id){
					chat.windowList.splice(chat.windowList.indexOf(Id),1);
					i--;
				}
			};
        };
		
        $scope.showChat = function(f) {
			if(f.Status < 0){
				return;
			}
			var Id=f.Id;
			// remove any old instances
			$scope.closeChat(Id);
			// add the new one
			chat.windowList.unshift(Id);
			console.log(chat.windowList);
			// temp overflow catch MAKE THIS BETTER LATER??!?!
			if(chat.windowList.length>3){
				chat.windowList.splice(3,chat.windowList.length-3);
			}
			// scroll the chat window
			function scroll2btm(){
				var divName="#chatWindow_messageLog_"+Id;
				if($(divName)==undefined){
					console.log("didn't find the message log container");
					return;
				}
				$(divName).stop().animate({scrollTop: $(divName)[0].scrollHeight}, 'fast');
			}
			setTimeout(scroll2btm,0);
			
        };
		
		$scope.sendMessage=function(Id,Name,Message){
			if(Message.length<1){
				//empty string, ignore
				return;
			}
			// add communication library code here
            chat.sendMessage(Id, Message);

			var Timestamp=0;
			
			// update local version (unless it is echo'd later, then remove this line
			$scope.receiveMessage(Id,Message,1);
		};
		
		$scope.receiveMessage=function(Id,Message,Myself){
			var Name=0;
			$scope.friends.forEach(function(f){
				if(f.Id==Id){
					Name=f.Name;
				}
			});
			if(!Name){
				// friend not found
				// update friend list
				chat.update();
				Name=0;
				$scope.friends.forEach(function(f){
					if(f.Id==Id){
						Name=f.Name;
					}
				});
				// if still not in list, error
				if(!Name){
					// log problem
					console.log("Couldn't recieve message from ID ["+ID+"]. It is missing from the friends list.");
					// and escape
					return;
				}
			}
			
			// find if the chat window for this friend is open
			var open=0;
			chat.windowList.forEach(function(id){
				if(id==Id){
					open=1;
				}
			});
			// if not, pop-up
			if(!open){
				$scope.showChat({Id:Id,Status:0});
				$scope.$apply();
				// scroll all the way upon the pop-up
				var divName="#chatWindow_messageLog_"+Id;
				$(divName).stop().animate({scrollTop: $(divName)[0].scrollHeight}, 0);
			}
			
			var displayName=Myself?"Me":Name;
			// add the message
				var msg={Name:displayName,Message:Message};
				var oldOne=0;
				$scope.chatLogs.forEach(function(cl){
					if(Id==cl.Id){
						oldOne=1;
						// add new
						cl.Messages.push(msg);
						// remove overflow
						if(cl.length>100){
							cl.splice(0,f.chatLog.length-100);
						}
					}
				});
				if(!oldOne){
					// make a new one
					$scope.chatLogs.push({Id:Id,Messages:[msg]});
				}
			// scroll the chat window
			// not sure if DOM exists yet... might be funky
			var divName="#chatWindow_messageLog_"+Id;
			$(divName).stop().animate({scrollTop: $(divName)[0].scrollHeight}, 800);
		};
		
        $scope.addFriend = function() {
            $scope.addFriendTextBoxError = null;
            $http.get('/api/social/addFriend/' + $scope.newFriendId)
                 .success(function(data, status, headers, config) {
                      if (data.Code == 0) {
                          $scope.newFriendId = '';
                          $scope.addFriendTextBox = false;
                      } else if (data.Code == 1) {
                          $scope.addFriendTextBoxError = 'Player does not exist';
                      } else if (data.Code == 2) {
                          $scope.addFriendTextBoxError = 'You are already friend with that person';
                      } else if (data.Code == 3) {
                          $scope.addFriendTextBoxError = 'You can not add yourself as a friend';
                      } else if (data.Code == 4) {
                          $scope.addFriendTextBoxError = 'The player you are trying to add is currently offline';
                      }
                 });
        };
	})
	
	// used to keep the scroll state persistant, and to keep the intervals from stacking
	.factory("homeScreenScroller",function(){
		return {persist:{}};
	})
	
    .controller('HomeControl', function($scope, $rootScope, $location, $http, user, homeScreenScroller) {
        $rootScope.title = "Home";
		
		$scope.user=user;
		$scope.announcements=announcementList;
        $http.get('/api/getAnnouncements')
             .success(function(data, status, headers, config) {
                 $scope.announcements = data;
             });
		
		if(user.Name){
			// if user is logged in
			function cycleAnnouncements(speed){
				// only cycle if the elements exist (ie: it is the home page)
				if($location.path()=="/"){
					// fit the end to the beginning, restart the loop
					if(homeScreenScroller.persist.index>=$scope.announcements.length){
						homeScreenScroller.persist.index=0;
						$("#home-announcementList").stop().animate({top:0},0);
					}
					// continue motion
					homeScreenScroller.persist.index++;
					var h=180;
					$("#home-announcementList").stop().animate({top:-h*homeScreenScroller.persist.index},speed);
				}
			}
			
			if(homeScreenScroller.persist.interval==undefined){
				// initialize
				homeScreenScroller.persist.index=-1;
				homeScreenScroller.persist.interval=window.setInterval(function(){cycleAnnouncements("slow");},3000);
			}
			else{
				// pre-scroll to old location
				// correct index addition
				homeScreenScroller.persist.index--;
				// delay until DOM has been created
				setTimeout(function(){cycleAnnouncements(0);},0);
			}
			

            $http.get('/api/game/result')
                 .success(function(data, status, headers, config) {
                     $scope.matches = data;
                 });
		}
		
    })
	
    .controller('LoginControl', function($scope, $rootScope, $http, $location, user, chat) {
        $rootScope.title = "Login";
        $("#register_now").click(function() {
            $("#register").show('fast');
            $(this).hide('fast');
        });
		
        $scope.login = function() {
            $scope.LoginEmailError = $scope.LoginPasswordError = null;
            $http.post('/api/login', 
                       $("#login").serialize(), 
                       {headers: {'Content-Type': 'application/x-www-form-urlencoded'}}
                      )
                 .success(function(data, status, headers, config) {
                      if (data.Code == 0) {
                          user.Name = data.Name;
                          user.FirstName = data.FirstName;
                          user.LastName = data.LastName;
                          user.Id = data.Id;
                          chat.update();
                          chat.openWSConnection();
                          $location.path('/');
                      } else if (data.Code == 1) { // user not found
                          $scope.LoginEmailError = 'Email does not exist';
                      } else if (data.Code == 2) { // incorrect password
                          $scope.LoginPasswordError = 'Incorrect password';
                      }
                 });
        };
		
		$scope.register = function() {
			if ($scope.Password != $scope.retypePassword) {
				$scope.RegPasswordError = 'Password mismatch';
				return;
			}
			$scope.RegEmailError = $scope.RegPasswordError = $scope.RegNameError = null;
			$http.post('/api/register', 
					   $("#register").serialize(), 
					   {headers: {'Content-Type': 'application/x-www-form-urlencoded'}}
					  )
				 .success(function(data, status, headers, config) {
					  if (data.Code == 0) {
                          window.location.replace('/');
					  } else if (data.Code == 1) {
						  $scope.RegEmailError = 'Email already exists';
					  } else if (data.Code == 2) {
						  $scope.RegNameError = 'Name already exists';
					  }
				 });
		};
		
    })
	
	
	
	.controller("ProfileControl",function($scope, $http, $routeParams, $location, user, chat) {
		var profileName=$routeParams.userID;
		// load user data from server
		// lookup user.Name == name	
		//var Id=1;// placeholder
		
        $scope.user = user;

		//$scope.profileUser=0;
		//if(Id>0){
			
			// placeholder
			//$scope.profileUser={Name:profileName,AvatarURL:"/avatar/"+Id};
			
		//}
		
		// load the game list
		$scope.gameList = null;
        $http.get('/api/game/list')
             .success(function(data, status, headers, config) {
                  if (data.Code == 0) {
                      $scope.gameList = data.Games;
                  }
             });
		$scope.selectedGame=0;

        $scope.toggleFriends = function(e, name, isFriend) {
            if (isFriend) {
                $http.get('/api/social/deleteFriend/' + name)
                     .success(function(data, status, headers, config) {
                          if (data.Code == 0) {
                              $scope.profile.IsFriend = false;
							  chat.update(true);
                          }
                     });
            } else {
                $http.get('/api/social/addFriend/' + name)
                     .success(function(data, status, headers, config) {
                          if (data.Code == 0) {
                              $(e.currentTarget).prop('disabled', true).text("Friend Request Sent");
                          } else if (data.Code == 4) {
                              alert("The person you are trying to add as friend is corrently offline");
                          } else if (data.Code != 0) {
                              alert("Something went wrong, please try again");
                          }
                     });
            }
        };
		
		
		$scope.tabs=[
			"Match History",
			"Game Statistics"
		];
		$scope.selectedTab=1;
		
		$scope.stats=[
			[],
			[]
		];
		
        $http.get('/api/profile/' + profileName)
             .success(function(data, status, headers, config) {
                  if (data.Code == 0) {
                      $scope.profile = data;
					  compileStatData();
					  
					  
                  } else {
                      alert('User ' + profileName + ' does not exist');
                      $location.path('/');
                      return;
                  }
             });
		/*
		// helper function for nice time formats
		function timeFormat(then,now){
			var delta=now-then;
			var lbl;
			var n;
			if(delta<1000){
				n=delta;
				lbl="ms";
			}
			else if(delta<1000*60){
				n=Math.floor(delta/1000*10)/10;
				lbl="s";
			}
			else if(delta<1000*60*60){
				n=Math.floor(delta/1000/60*10)/10;
				lbl="m";
			}
			else if(delta<1000*60*60){
				n=Math.floor(delta/1000/60/60*10)/10;
				lbl="h";
			}
			else{
				n=Math.floor(delta/1000/60/60/24*10)/10;
				lbl="days";
			}
			return n+" "+lbl;
		};
		var now=new Date().getTime();
		$scope.stats.forEach(function(s){
			s.forEach(function(ss){
				ss.deltaTime=timeFormat(0,ss.time);
				ss.deltaDate=timeFormat(ss.date,now);
			});
		});
		*/
		
		
		function compileStatData(){
			function getPercent(num,den){
				if(!den || num==undefined || num=="NaN"){
					return "unknown";
				}
				return Math.floor(num/den*10000)/100+"%";
			}
		
			function HistoryNote(gameName,result){
				var lut=["Tie","Win","Loss","Disconnect"];
				return {
					name:gameName,
					result:lut[result],
					resultIndex:result
				};
			}
			
			function StatsNote(gameName){
				return {
					name:gameName,
					wins:0,
					losses:0,
					disconnects:0,
					ties:0,
					total:0
				};
			};
			
			function compileHistoryAndStatistics(arr){
				var db={};
				var historyList=[];
				arr.forEach(function(e){
					console.log(e);
					var n=e.Name;
					var r=e.Result;
					// history
					historyList.push(HistoryNote(n,r));
					
					// stats compilation
					if(db[n]==undefined){
						console.log("NEW");
						db[n]=StatsNote(n);
					}
					else{
						console.log("OLD");
					}
					switch(r){
						case 0:
							db[n].ties++;
						break;
						case 1:
							db[n].wins++;
						break;
						case 2:
							db[n].losses++;
						break;
						case 3:
							db[n].disconnects++;
						break;
					}
					db[n].total++;
				});
				var dbList=[];
				for(var name in db){
					var e=db[name];
					e.winPerc=getPercent(e.wins,e.total);
					dbList.push(e);
				}
				console.log(dbList);
				return {db:dbList,hl:historyList};
			};
			
			var output=compileHistoryAndStatistics($scope.profile.Games);
			
			$scope.stats=[
				output.hl,
				output.db
			];
			
		}	
			// TODO TEMP DEBUG CODE HERE
			/*
			var output=compileHistoryAndStatistics(
				[
					{
					  "Result": 2,
					  "Name": "test"
					},
					{
					  "Result": 2,
					  "Name": "2"
					},
					{
					  "Result": 2,
					  "Name": "teasdfst"
					},
					{
					  "Result": 2,
					  "Name": "asdf"
					},
					{
					  "Result": 2,
					  "Name": "13241234"
					},
					{
					  "Result": 2,
					  "Name": "A:KLJ#@IJ()@#"
					},
					{
					  "Result": 1,
					  "Name": "aaaa"
					},
					{
					  "Result": 2,
					  "Name": "test"
					},
					{
					  "Result": 1,
					  "Name": "2"
					},
					{
					  "Result": 1,
					  "Name": "teasdfst"
					},
					{
					  "Result": 1,
					  "Name": "asdf"
					},
					{
					  "Result": 1,
					  "Name": "13241234"
					},
					{
					  "Result": 1,
					  "Name": "A:KLJ#@IJ()@#"
					},
					{
					  "Result": 1,
					  "Name": "aaaa"
					},
					{
					  "Result": 0,
					  "Name": "teasdfst"
					},
					{
					  "Result": 0,
					  "Name": "asdf"
					},
					{
					  "Result": 3,
					  "Name": "teasdfst"
					},
					{
					  "Result": 3,
					  "Name": "asdf"
					},
				]
			);
			*/
			
			// TODO
		
		
		
		
	})
	
	
	.controller("CreationControl",function($scope, $http, $location, $rootScope) {
        $scope.gameList = null;
        $http.get('/api/game/list')
             .success(function(data, status, headers, config) {
                  if (data.Code == 0) {
                      $scope.gameList = data.Games;
                  }
             });
		
		$scope.selectedGame=0;
		
		// add some object to contain and track the form attributes
		// then send it off to some server process to put you into a waiting queue
		// then when the response comes in, shuttle the user to the correct game lobby
		
		$scope.createGame = function() {
			$http.post('/api/game/create', 
					   $.param({'roomName': $scope.roomName, 'gameId': $scope.selectedGame, 'password': $scope.roomPassword, 'capacity': $scope.roomCapacity}),
					   {headers: {'Content-Type': 'application/x-www-form-urlencoded'}}
					  )
				 .success(function(data, status, headers, config) {
					  if (data.Code == 0) {
                          $rootScope.roomPassword = $scope.roomPassword; // ugly, should use service
                          $location.path('/game/' + data.RoomId);
					  } 
                 });
        };
	})
	
	.controller("GameControl",function($scope, $routeParams, chat, game, user, $http, $location, $rootScope) {
		
		/*
		// fix the iframe size
		// too FREEKING annoying. datong made the bad iframe, he can fix it.
		$('#game-iframe iframe').contents().find('body').css({"min-height":"100","overflow":"hidden"});
		function fixScreen(){
			function getCB(){
				var first=1;
				return function(){
					var offset=40*first;
					var h=$('#game-iframe iframe').contents().find('body').height()+offset;
					if(first){
						console.log("FIRST");
						console.log(h);
					}
					$('#game-iframe').height(h);
					$('#game-iframe iframe').height(h);
					first=0;
				}
			}
			setInterval(getCB(),100);
		}
		setTimeout(fixScreen,1000);
		*/
		
		var roomId = $routeParams.roomId;
		
		$scope.chat=chat;
		
		$scope.playerIsFriend=function(player){
            if (player.Id == user.Id) { // self
                return true;
            }
            for (i=0; i<chat.friends.length; i++) {
                if (chat.friends[i].Id == player.Id) {
                    return true;
                }
            }
            return false;
		}
		$scope.friendIsInGame=function(f){
			for(var i=0;i<$scope.playerList.length;i++){
				var p=$scope.playerList[i];
				if(p.Id==f.Id){
					return 1;
				}
			}
			return 0;
		}

        $scope.invite = function() {
            var $input = $('#inviteSearch');
            $input.typeahead('close');
            var input = $.trim($input.val()); // have to do this since ng-model does not detect changes made by typeahead.js
            $http.get('/api/social/gameInvite/' + roomId + '/' + input)
                 .success(function(data, status, headers, config) {
                      if (data.Code == 0) {
                          $scope.inviteFriendTextBox= 'Invitation sent';
                          $scope.inviteFriendTextBoxError = false;
                      } else if (data.Code == 1) {
                          $scope.inviteFriendTextBox = 'Player does not exist';
                          $scope.inviteFriendTextBoxError = true;
                      } else if (data.Code == 2) {
                          $scope.inviteFriendTextBox = 'Player is currently busy';
                          $scope.inviteFriendTextBoxError = true;
                      } else if (data.Code == 3) {
                          $scope.inviteFriendTextBox = 'You can not send invitation to yourself';
                          $scope.inviteFriendTextBoxError = true;
                      } else if (data.Code == 4) {
                          $scope.inviteFriendTextBox = 'Player is currently offline';
                          $scope.inviteFriendTextBoxError = true;
                      } else if (data.Code == 5) {
                          $scope.inviteFriendTextBox = 'Room does not exist';
                          $scope.inviteFriendTextBoxError = true;
                      }
                 });
        };

        $scope.addFriend = function(e, name) {
            $http.get('/api/social/addFriend/' + name)
                 .success(function(data, status, headers, config) {
                      if (data.Code != 0) {
                          alert("Something went wrong, please try again");
                      } else {
                      console.log(e);
                          $(e.currentTarget).prop('disabled', true);
                      }
                 });
        };
		
		// main models for DOM
		$scope.messages=[];
		function addMessage(name,msg){
			var d=new Date();
			var arr=d.toTimeString().split(" ")[0].split(":");
			var h=arr[0]%12;
			var m=arr[1];
			var s=arr[2];
			var tstr=h+":"+m+":"+s;//+" "+(am?"am":"pm");
			
			var myself=(name==user.Name);
			console.log("["+myself+"] Name: "+name+", UserName: "+user.Name);
			
			var obj={
				Name:name,
				Message:msg,
				Timestamp:tstr,
				Self:myself,
				Highlight:1
			};
			$scope.messages.push(obj);
			
			// limit number of messages kept
			var MAX=100;
			if($scope.messages.length>MAX){
				$scope.messages.splice(0,$scope.messages.length-MAX);
			}
			
			function cb(o){
				return function(){
					o.Highlight=0;
					$scope.$apply();
				};
			}
			setTimeout(cb(obj),500);
		}
		$scope.playerList=[];
		
		// communication
        $scope.sendChatMessage = function(msg) {
            game.sendChatMessage(msg);
			addMessage(user.Name,msg);
			$("#game-chatScroll").stop().animate({scrollTop: $("#game-chatScroll")[0].scrollHeight}, 800);
        };
        game.setChatHandler(function(from, msg) {
            //console.log("from ", from, "Msg", msg);
			var name=0;
			$scope.playerList.forEach(function(p){
				if(p.Id==from){
					name=p.Name;
				}
			});
			if(!name){
				console.log("Unknown player ID ["+from+"]");
				return;
			}
			addMessage(name,msg);
			$scope.$apply();
			$("#game-chatScroll").stop().animate({scrollTop: $("#game-chatScroll")[0].scrollHeight}, 800);
        });
		game.setJoinHandler(function(from,name){
			$scope.playerList.push({Id:from,Name:name});
			$scope.$apply();

            if (playerListChangeHandler) {
                playerListChangeHandler($scope.playerList);
            }
		});
		game.setLeaveHandler(function(from,none){
			for(var i=0;i<$scope.playerList.length;i++){
				var p=$scope.playerList[i];
				if(p.Id==from){
					$scope.playerList.splice(i,1);
					break;
				}
			};
			$scope.$apply();

            if (playerListChangeHandler) {
                playerListChangeHandler($scope.playerList);
            }
		});

        function joinRoom(password) {
            var url = null;
            if (password) {
                url = '/api/game/getInfo/' + roomId + '/' + password;
            } else {
                url = '/api/game/getInfo/' + roomId;
            }
            $http.get(url)
                 .success(function(data, status, headers, config) {
                      if (data.Code == 0) {
                          $scope.roomId = data.RoomId;
                          $scope.gameName = data.GameName;
                          $scope.gameURL = data.GameURL;
                          $scope.roomName = data.RoomName;
                          $scope.passwordProtected = data.PasswordProtected;
		                  $scope.maxCapacity = data.Capacity;

                          $rootScope.roomPassword = '';

                          game.openWSConnection(roomId, function() {
                              // game injection
                              $('#game-iframe iframe').attr('src', $scope.gameURL).load(function() {
                                  $(this).get(0).contentWindow.main(platform);
                                  playerListChangeHandler($scope.playerList);
                              });
                          });

                          $scope.$on('$locationChangeStart', function (event, next, current) {
                              var answer = confirm("Are you sure you want to leave this room?");
                              if (!answer) {
                                  event.preventDefault();
                                  return;
                              }
                              game.closeWSConnection();
                          });

                      } else if (data.Code == 1) { // game does not exist
                          alert("Game does not exist");
                          $location.path('/');
                      } else if (data.Code == 2) { // need password
                          var pwd = prompt("Please enter the room password");
                          if (!pwd) {
                              alert('Youm must provide a password to join this room');
                              $location.path('/');
                              return;
                          }
                          joinRoom(pwd);
                      } else if (data.Code == 3) { // started
                          alert('The game you are trying to join already started');
                          $location.path('/');
                          return;
                      } else if (data.Code == 4) { // full
                          alert('The game you are trying to join already at max capacity');
                          $location.path('/');
                          return;
                      }
                 });
        }

        $scope.typeahead = function() {
            var $tmp = $('#inviteSearch').typeahead({
                hint: true,
                highlight: true,
                minLength: 1
            },
            {
                name: 'friends',
                displayKey: 'name',
                source: function (n, cb) {
                    var matches = [];
                    var reg = new RegExp(n, 'i');

                    $.each(chat.friends, function(i, f) {
                        var tagName = f.FirstName + ' "' + f.Name + '" ' + f.LastName;
                        if (reg.test(tagName)) {
                            matches.push({'tag': tagName, 'name': f.Name, 'key': n});
                        }
                    });

                    cb(matches);
                },
                templates: {
                    empty: function(m) { return '<div class="empty-message">Hit Enter to invite player <strong>' + m.query + '</strong></div>'},
                    suggestion: function(m) {return '<p><strong>' + m.name + '</strong> – ' + m.tag + '</p>'}
                }
            });
        };

        var playerListChangeHandler = null;

        var platform = {
            'TIE': 0,
            'WIN': 1,
            'start': function() {
                game.startGame();
            },
            'send': function(obj) {
                game.sendGameMessage(obj);
            },
            'reportResult': function(type, winner) {
                console.log('Game report', type, winner);
                game.reportResult(type, winner);
            },
            'setOnMessageCallback': function(cb) {
                game.setGameMessageHandler(cb);
            },
            'setStartCallback': function(cb) {
                game.setStartHandler(function() {
                    cb(user.Id, $scope.playerList);
                });
            },
            'setSwitchMasterCallback': function(cb) {
                game.setSwitchMasterHandler(cb);
            },
            'setPlayerListChangeCallback': function(cb) {
                playerListChangeHandler = cb;
            }
        };

        if ($rootScope.roomPassword) {
            joinRoom($rootScope.roomPassword);
        } else {
            joinRoom();
        }

	    })
	
	.controller("SettingsControl",function($scope, user, $http) {
        var tempUser = {};
        $.extend(tempUser, user);
        $scope.User = tempUser;
        $scope.NameError = null;

        $scope.checkIfNameExists = function(name) {
            if (user.Name == name) { // do not check if the user is using his own name
                $scope.NameError = null;
                return;
            }
            $http.get('/api/user/nameExists?Name=' + encodeURIComponent(name))
                 .success(function(data, status, headers, config) {
                      if (data.Code == 0) {
                          $scope.NameError = null;
                      } else {
                          $scope.NameError = 'Player name already exists';
                      }
                 });
        };
	});
	
	
	
	
	/**
		Junk data for building the angular logic.
		Load real data later on.
	*/
	
	var gameList=[
		"Chess",
		"Black jack",
		"Hearts",
		"Checkers",
		"Dominoes",
		"Clue",
		"Battleship",
		"Connect4",
		"1zt4tqACqs",
		"JparuKczik",
		"Z3oqEVY0Qi",
		"A Bad Pacman Clone",
		"Super Stolee Bros"
	];
	var playerStat=function(name,wins,losses,time,date){
		return {name:name,wins:wins,losses:losses,time:time,date:date};
	}
	var todayK=new Date().getTime();
	var dayK=1000*60*60*24;
	var playerHistory=[
		//		name	win?	loss?	time of game	getTime when it started/finished
		playerStat(2,	0,		1,		38100,		todayK-dayK*.5),
		playerStat(2,	1,		0,		18500,		todayK-dayK*3.81),
		playerStat(8,	0,		1,		7400,		todayK-dayK*.45),
		playerStat(8,	1,		0,		23400,		todayK-dayK*1.32),
		playerStat(8,	0,		1,		400,		todayK-dayK*.74),
		playerStat(9,	1,		0,		4800,		todayK-dayK*4.2),
		playerStat(12,	0,		1,		42300,		todayK-dayK*.3),
		playerStat(13,	1,		0,		000,		todayK-dayK*.6)
	];
	
	var playerStats=[
		//		name	wins	losses	aggregate time		last played??
		playerStat(2,	10,		20,		1234567,	todayK-dayK*.55),
		playerStat(8,	5,		1,		9827378,	todayK-dayK*1.3),
		playerStat(9,	526,	13,		1275349,	todayK-dayK*3.1),
		playerStat(12,	0,		323,	5036818,	todayK-dayK*2.8),
		playerStat(13,	4,		0,		0000000,	todayK-dayK*50.1)
	];
	
	
	/*
	var friend=function(Id, FirstName, LastName, Name, Signature, Status) {
		return {
			// query properties
			Id:Id,
			FirstName: FirstName,
			LastName: LastName,
			Name: Name,
			Signature: Signature,
			Status: Status,
			chatLog:[
				// debug listing
				chatLogMessage("Me","HELLO THERE",0),
				chatLogMessage(Name,"How",0),
				chatLogMessage(Name,"are you doing this fine day",0),
				chatLogMessage(Name,"hmmmmmmmmmmmmmmmmmmmm?",0)
			],
			addMessage:function(name,msg,ts){
				// add new
				this.chatLog.push(chatLogMessage(name,msg,ts));
				// remove overflow
				if(this.chatLog.length>100){
					this.chatLog.splice(0,this.chatLog.length-100);
				}
			}
		};
	}
	*/
	/*
	var friendList=[
		friend(11,"Peter", "Parker","TheHumanSpider","Spidering", 0),
		friend(12,"Clark", "Kent","ThatOneJournalist","Supering", 0),
		friend(13,"Bruce", "Wayne","IAMBATMAN","Detectiving", 1),
		friend(14,"Tony", "Stark","FEman","I'm the best. mirite?", 1),
		friend(15,"John", "Smith","WhoExactly?","Fantastic!", 2),
	];
	*/
	
	var Announcement=function(title,msg){
		return {title:title,message:msg};
	}
	var announcementList=[
		Announcement("Development...","is underway!"),
		Announcement("Announcements Added","Animations have been added."),
		Announcement("Interval Bugs","have been patched. It's looking good.")
		//Announcement("Test","Four"),
		//Announcement("Test","Five"),
		//Announcement("Test","Six"),
		//Announcement("Test","Seven"),
		//Announcement("Test","Eight"),
		//Announcement("Test","Nine"),
		//Announcement("Test","Ten")
	];
	/**
			END
	*/
	
	
})();
