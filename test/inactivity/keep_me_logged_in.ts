import Bluebird = require("bluebird");
import loginAndRegisterTotp from "../helpers/login-and-register-totp";
import VisitPage from "../helpers/visit-page";
import FillLoginPageWithUserAndPasswordAndClick from "../helpers/fill-login-page-and-click";
import WithDriver from "../helpers/with-driver";
import ValidateTotp from "../helpers/validate-totp";
import WaitRedirected from "../helpers/wait-redirected";

describe("Keep me logged in", function() {
  this.timeout(15000);
  WithDriver();

  before(function() {
    const that = this;
    return loginAndRegisterTotp(this.driver, "john")
      .then(function(secret: string) {
        that.secret = secret;
        if(!secret) return Bluebird.reject(new Error("No secret!"));
        return Bluebird.resolve();
      });
  });

  it("should disconnect user after inactivity period", function() {
    const that = this;
    const driver = this.driver;
    return VisitPage(driver, "https://login.example.com:8080/?rd=https://admin.example.com:8080/secret.html")
    .then(() => FillLoginPageWithUserAndPasswordAndClick(driver, 'john', 'password', false))
    .then(() => ValidateTotp(driver, that.secret))
    .then(() => WaitRedirected(driver, "https://admin.example.com:8080/secret.html"))
    .then(() => VisitPage(driver, "https://home.example.com:8080/"))
    .then(() => driver.sleep(3000))
    .then(() => driver.get("https://admin.example.com:8080/secret.html"))
    .then(() => WaitRedirected(driver, "https://login.example.com:8080/?rd=https://admin.example.com:8080/secret.html"))
  });

  it.only("should keep user logged in after inactivity period", function() {
    const that = this;
    const driver = this.driver;
    return VisitPage(driver, "https://login.example.com:8080/?rd=https://admin.example.com:8080/secret.html")
    .then(() => FillLoginPageWithUserAndPasswordAndClick(driver, 'john', 'password', true))
    .then(() => ValidateTotp(driver, that.secret))
    .then(() => WaitRedirected(driver, "https://admin.example.com:8080/secret.html"))
    .then(() => VisitPage(driver, "https://home.example.com:8080/"))
    .then(() => driver.sleep(5000))
    .then(() => driver.get("https://admin.example.com:8080/secret.html"))
    .then(() => WaitRedirected(driver, "https://admin.example.com:8080/secret.html"))
  });
});