go build
copy Peerster.exe usr0
copy Peerster.exe usr1
copy Peerster.exe usr2
copy Peerster.exe usr3
copy Peerster.exe usr4
copy Peerster.exe usr5
copy Peerster.exe usr6
python prepare.py 127.0.0.1

move usr0.bat usr0
move usr1.bat usr1
move usr2.bat usr2
move usr3.bat usr3