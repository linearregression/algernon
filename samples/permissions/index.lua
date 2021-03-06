-- From the example for https://github.com/xyproto/permissions2
print("Has user bob: " .. tostring(HasUser("bob")))
print("Logged in on server: " .. tostring(IsLoggedIn("bob")))
print("Is confirmed: " .. tostring(IsConfirmed("bob")))
print("Username stored in cookie (or blank): " .. Username())
print("Current user is logged in, has a valid cookie and *user rights*: " .. tostring(UserRights()))
print("Current user is logged in, has a valid cookie and *admin rights*: " .. tostring(AdminRights()))
print("Try: /register, /confirm, /remove, /login, /logout, /clear, /data, /makeadmin and /admin")

if urlpath() ~= "/" then
  print()
  print[[NOTE: The current URL path is not "/". For the default URL permissions to work, Algernon must either be run from this directory, or the URL prefixes must be configured correctly.]]
end
