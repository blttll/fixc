# fixc

fixc is a simple Financial Information eXchange (FIX) protocol console client. 
fixc connects to remote host and sends out FIX messages it reads from local
scenario file.

Apart from FIX messages two commands can be used in scenario file:
* `sleep <time>`
* `exit`

fixc logs all messages and commands it executes to stdout.

## Log format

```
...
20130818-14:08:04.375 - sleeping for 2s
20130818-14:08:04.376 < 8=FIX.4.3|9=59|35=0|49=MySender|56=MyTarg...
^                     ^ ^
|                     | |
|                     | +--- FIX message or command
|                     +----- Direction (< - for out)
+--------------------------- Local timestamp (UTC)
```

## Example scenario file

```
8=FIX.4.3|9=111|35=A|49=q|56=demo|34=1|52=20130807-13:35:05|98=0|108=30|141=Y|553=spot|554=come1|10=246|
sleep 2s
8=FIX.4.3|9=166|35=V|49=q|56=demo|34=2|52=20130807-13:35:05|50=T|128=ALL|262=AUDCAD|263=1|264=0|265=1|266=Y|146=1|55=AUD/CAD|460=4|267=2|269=0|269=1|10=053|
sleep 1m
exit
```
Just copy-paste your own messages with "|" as delimiter one per line. Don't 
worry about tags 8, 10, 34, 49, 52, 56 - fixc replaces them.

## Usage

```
$ ./fixc
Usage of fixc:
  -b=30s: HeartBeat
  -f="input.log": Input file
  -h="": Target host
  -p="": Target port
  -s="MySender": SenderCompID
  -t="MyTarget": TargetCompID
  -v="4.3": FIX protocol version
  -x=false: Use TLS
```

## Binaries
* https://github.com/blttll/fixc/releases

## Mini-FIX
* fixc was inspired by Mini-FIX (http://elato.se/minifix/)
