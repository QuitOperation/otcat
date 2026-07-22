// A second independent Modbus TCP server, this time in JavaScript
// (modbus-serial, npm's most widely used Modbus package), giving
// otcat's Go client a third-party target implemented by yet another
// author in yet another language. Agreement across Python, JS, and
// otcat's own Go mock is real cross-implementation evidence.
const ModbusRTU = require("modbus-serial");

const holding = new Uint16Array(400);
holding[0] = 0x1234;
holding[10] = 10; holding[11] = 20; holding[12] = 30;
holding[200] = 0x4049; holding[201] = 0x0fdb;

const coils = new Array(100).fill(false);
coils[0] = true;
coils[3] = true;

const discrete = new Array(100).fill(false);
discrete[7] = true;

const inputRegs = new Uint16Array(100);
inputRegs[5] = 777;

const vector = {
    getHoldingRegister: (addr) => holding[addr],
    setRegister: (addr, value) => { holding[addr] = value; },
    getInputRegister: (addr) => inputRegs[addr],
    getCoil: (addr) => coils[addr],
    setCoil: (addr, value) => { coils[addr] = value; },
    getDiscreteInput: (addr) => discrete[addr],
};

const server = new ModbusRTU.ServerTCP(vector, { host: "127.0.0.1", port: 15031, unitID: 1 });
server.on("initialized", () => {
    console.log("modbus-serial (Node.js) interop server listening on 127.0.0.1:15031");
});
server.on("error", (e) => {
    console.error("server error:", e);
});

// Self-terminate so this never becomes an orphaned process in the
// test sandbox -- not something a real deployment would do.
setTimeout(() => process.exit(0), 20000);
