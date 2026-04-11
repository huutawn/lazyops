"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.createClient = createClient;
exports.discoverServices = discoverServices;
exports.connectDB = connectDB;
exports.connectRedis = connectRedis;
const client_1 = require("./client");
/**
 * Create a configured LazyOps client
 */
function createClient(config) {
    return new client_1.LazyOpsClient(config);
}
/**
 * Discover all services available in the current environment
 */
function discoverServices(config) {
    const client = new client_1.LazyOpsClient(config);
    return client.discoverServices();
}
/**
 * Get a database connection string for the specified internal service
 * Supports postgres, mysql, and other SQL databases
 */
function connectDB(kind, config) {
    const client = new client_1.LazyOpsClient(config);
    return client.getInternalServiceConnectionString(kind);
}
/**
 * Get a Redis connection string
 */
function connectRedis(config) {
    const client = new client_1.LazyOpsClient(config);
    return client.getInternalServiceConnectionString('redis');
}
