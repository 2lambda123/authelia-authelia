
import sinon = require("sinon");
import BluebirdPromise = require("bluebird");
import assert = require("assert");
import U2FRegisterPost = require("../../../../../src/lib/routes/secondfactor/u2f/register/post");
import AuthenticationSession = require("../../../../../src/lib/AuthenticationSession");
import ExpressMock = require("../../../../mocks/express");
import { UserDataStoreStub } from "../../../../mocks/storage/UserDataStoreStub";
import { ServerVariablesMockBuilder, ServerVariablesMock } from "../../../../mocks/ServerVariablesMockBuilder";
import { ServerVariables } from "../../../../../src/lib/ServerVariables";


describe("test u2f routes: register", function () {
  let req: ExpressMock.RequestMock;
  let res: ExpressMock.ResponseMock;
  let mocks: ServerVariablesMock;
  let vars: ServerVariables;

  beforeEach(function () {
    req = ExpressMock.RequestMock();
    req.app = {};
    req.session = {
      auth: {
        userid: "user",
        first_factor: true,
        second_factor: false,
        identity_check: {
          challenge: "u2f-register",
          userid: "user"
        }
      }
    };
    req.headers = {};
    req.headers.host = "localhost";

    const s = ServerVariablesMockBuilder.build();
    mocks = s.mocks;
    vars = s.variables;

    const options = {
      inMemoryOnly: true
    };

    mocks.userDataStore.saveU2FRegistrationStub.returns(BluebirdPromise.resolve({}));
    mocks.userDataStore.retrieveU2FRegistrationStub.returns(BluebirdPromise.resolve({}));

    res = ExpressMock.ResponseMock();
    res.send = sinon.spy();
    res.json = sinon.spy();
    res.status = sinon.spy();
  });

  describe("test registration", test_registration);


  function test_registration() {
    it("should save u2f meta and return status code 200", function () {
      const expectedStatus = {
        keyHandle: "keyHandle",
        publicKey: "pbk",
        certificate: "cert"
      };
      mocks.u2f.checkRegistrationStub.returns(BluebirdPromise.resolve(expectedStatus));

      return AuthenticationSession.get(req as any, vars.logger)
        .then(function (authSession) {
          authSession.register_request = {
            appId: "app",
            challenge: "challenge",
            keyHandle: "key",
            version: "U2F_V2"
          };
          return U2FRegisterPost.default(vars)(req as any, res as any);
        })
        .then(function () {
          return AuthenticationSession.get(req as any, vars.logger);
        })
        .then(function (authSession) {
          assert.equal("user", mocks.userDataStore.saveU2FRegistrationStub.getCall(0).args[0]);
          assert.equal(authSession.identity_check, undefined);
        });
    });

    it("should return error message on finishRegistration error", function () {
      mocks.u2f.checkRegistrationStub.returns({ errorCode: 500 });

      return AuthenticationSession.get(req as any, vars.logger)
        .then(function (authSession) {
          authSession.register_request = {
            appId: "app",
            challenge: "challenge",
            keyHandle: "key",
            version: "U2F_V2"
          };

          return U2FRegisterPost.default(vars)(req as any, res as any);
        })
        .then(function () { return BluebirdPromise.reject(new Error("It should fail")); })
        .catch(function () {
          assert.equal(200, res.status.getCall(0).args[0]);
          assert.deepEqual(res.send.getCall(0).args[0], {
            error: "Operation failed."
          });
          return BluebirdPromise.resolve();
        });
    });

    it("should return error message when register_request is not provided", function () {
      mocks.u2f.checkRegistrationStub.returns(BluebirdPromise.resolve());
      return AuthenticationSession.get(req as any, vars.logger)
        .then(function (authSession) {
          authSession.register_request = undefined;
          return U2FRegisterPost.default(vars)(req as any, res as any);
        })
        .then(function () { return BluebirdPromise.reject(new Error("It should fail")); })
        .catch(function () {
          assert.equal(200, res.status.getCall(0).args[0]);
          assert.deepEqual(res.send.getCall(0).args[0], {
            error: "Operation failed."
          });
          return BluebirdPromise.resolve();
        });
    });

    it("should return error message when no auth request has been initiated", function () {
      mocks.u2f.checkRegistrationStub.returns(BluebirdPromise.resolve());
      return AuthenticationSession.get(req as any, vars.logger)
        .then(function (authSession) {
          authSession.register_request = undefined;
          return U2FRegisterPost.default(vars)(req as any, res as any);
        })
        .then(function () { return BluebirdPromise.reject(new Error("It should fail")); })
        .catch(function () {
          assert.equal(200, res.status.getCall(0).args[0]);
          assert.deepEqual(res.send.getCall(0).args[0], {
            error: "Operation failed."
          });
          return BluebirdPromise.resolve();
        });
    });

    it("should return error message when identity has not been verified", function () {
      return AuthenticationSession.get(req as any, vars.logger)
        .then(function (authSession) {
          authSession.identity_check = undefined;
          return U2FRegisterPost.default(vars)(req as any, res as any);
        })
        .then(function () { return BluebirdPromise.reject(new Error("It should fail")); })
        .catch(function () {
          assert.equal(200, res.status.getCall(0).args[0]);
          assert.deepEqual(res.send.getCall(0).args[0], {
            error: "Operation failed."
          });
          return BluebirdPromise.resolve();
        });
    });
  }
});

