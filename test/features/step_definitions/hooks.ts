import Cucumber = require("cucumber");
import fs = require("fs");
import BluebirdPromise = require("bluebird");
import ChildProcess = require("child_process");
import { UserDataStore } from "../../../server/src/lib/storage/UserDataStore";
import { CollectionFactoryFactory } from "../../../server/src/lib/storage/CollectionFactoryFactory";
import { MongoConnector } from "../../../server/src/lib/connectors/mongo/MongoConnector";
import { IMongoClient } from "../../../server/src/lib/connectors/mongo/IMongoClient";
import { TotpHandler } from "../../../server/src/lib/authentication/totp/TotpHandler";
import Speakeasy = require("speakeasy");

Cucumber.defineSupportCode(function ({ setDefaultTimeout }) {
  setDefaultTimeout(20 * 1000);
});

Cucumber.defineSupportCode(function ({ After, Before }) {
  const exec = BluebirdPromise.promisify<any, any>(ChildProcess.exec);

  After(function () {
    return this.driver.quit();
  });

  function createRegulationConfiguration(): BluebirdPromise<void> {
    return exec("\
    cat config.template.yml | \
    sed 's/find_time: [0-9]\\+/find_time: 15/' | \
    sed 's/ban_time: [0-9]\\+/ban_time: 4/' > config.test.yml \
    ");
  }

  function createInactivityConfiguration(): BluebirdPromise<void> {
    return exec("\
    cat config.template.yml | \
    sed 's/expiration: [0-9]\\+/expiration: 10000/' | \
    sed 's/inactivity: [0-9]\\+/inactivity: 5000/' > config.test.yml \
    ");
  }

  function declareNeedsConfiguration(tag: string, cb: () => BluebirdPromise<void>) {
    Before({ tags: "@needs-" + tag + "-config", timeout: 20 * 1000 }, function () {
      return cb()
        .then(function () {
          return exec("./scripts/example-commit/dc-example.sh -f docker-compose.test.yml up -d authelia && sleep 1");
        })
    });

    After({ tags: "@needs-" + tag + "-config", timeout: 20 * 1000 }, function () {
      return exec("rm config.test.yml")
        .then(function () {
          return exec("./scripts/example-commit/dc-example.sh up -d authelia && sleep 1");
        });
    });
  }

  declareNeedsConfiguration("regulation", createRegulationConfiguration);
  declareNeedsConfiguration("inactivity", createInactivityConfiguration);

  function registerUser(context: any, username: string) {
    let secret: Speakeasy.Key;
    const mongoConnector = new MongoConnector("mongodb://localhost:27017/authelia");
    return mongoConnector.connect()
      .then(function (mongoClient: IMongoClient) {
        const collectionFactory = CollectionFactoryFactory.createMongo(mongoClient);
        const userDataStore = new UserDataStore(collectionFactory);

        const generator = new TotpHandler(Speakeasy);
        secret = generator.generate();
        return userDataStore.saveTOTPSecret(username, secret);
      })
      .then(function () {
        context.totpSecrets["REGISTERED"] = secret.base32;
      });
  }

  function declareNeedRegisteredUserHooks(username: string) {
    Before({ tags: "@need-registered-user-" + username, timeout: 15 * 1000 }, function () {
      return registerUser(this, username);
    });

    After({ tags: "@need-registered-user-" + username, timeout: 15 * 1000 }, function () {
      this.totpSecrets["REGISTERED"] = undefined;
    });
  }

  function needAuthenticatedUser(context: any, username: string): BluebirdPromise<void> {
    return context.visit("https://auth.test.local:8080/logout")
      .then(function () {
        return context.visit("https://auth.test.local:8080/");
      })
      .then(function () {
        return registerUser(context, username);
      })
      .then(function () {
        return context.loginWithUserPassword(username, "password");
      })
      .then(function () {
        return context.useTotpTokenHandle("REGISTERED");
      })
      .then(function () {
        return context.clickOnButton("TOTP");
      });
  }

  function declareNeedAuthenticatedUserHooks(username: string) {
    Before({ tags: "@need-authenticated-user-" + username, timeout: 15 * 1000 }, function () {
      return needAuthenticatedUser(this, username);
    });

    After({ tags: "@need-authenticated-user-" + username, timeout: 15 * 1000 }, function () {
      this.totpSecrets["REGISTERED"] = undefined;
    });
  }

  function declareHooksForUser(username: string) {
    declareNeedRegisteredUserHooks(username);
    declareNeedAuthenticatedUserHooks(username);
  }

  const users = ["harry", "john", "bob", "blackhat"];
  users.forEach(declareHooksForUser);
});