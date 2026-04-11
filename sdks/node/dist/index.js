"use strict";
/**
 * lazyops-sdk - LazyOps Service Discovery and Communication SDK
 *
 * Provides clean APIs for discovering and communicating with services
 * in a LazyOps deployment, both in local development and production.
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.connectRedis = exports.connectDB = exports.discoverServices = exports.createClient = exports.LazyOpsClient = void 0;
var client_1 = require("./client");
Object.defineProperty(exports, "LazyOpsClient", { enumerable: true, get: function () { return client_1.LazyOpsClient; } });
var helpers_1 = require("./helpers");
Object.defineProperty(exports, "createClient", { enumerable: true, get: function () { return helpers_1.createClient; } });
Object.defineProperty(exports, "discoverServices", { enumerable: true, get: function () { return helpers_1.discoverServices; } });
Object.defineProperty(exports, "connectDB", { enumerable: true, get: function () { return helpers_1.connectDB; } });
Object.defineProperty(exports, "connectRedis", { enumerable: true, get: function () { return helpers_1.connectRedis; } });
