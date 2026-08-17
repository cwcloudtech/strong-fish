import 'api_client.dart';
import '../models/models.dart';

/// The API surface, grouped by domain. Mirrors sf-ui's api/services.js so both
/// clients call the same endpoints in the same shapes.
class SfApi {
  final ApiClient client;

  SfApi(this.client);

  Map<String, dynamic> _map(dynamic data) => data is Map<String, dynamic> ? data : const {};

  List<Map<String, dynamic>> _list(dynamic data) =>
      (data as List? ?? const []).whereType<Map<String, dynamic>>().toList();

  // --- config & auth ---

  Future<Map<String, dynamic>> config() async => _map((await client.dio.get('/config')).data);

  Future<LoginResult> login(String email, String password) async {
    final response = await client.dio.post('/users/login', data: {'email': email, 'password': password});
    return LoginResult.fromJson(_map(response.data));
  }

  Future<LoginResult> register({
    required String email,
    required String password,
    required String name,
    required String surname,
    required String locale,
  }) async {
    final response = await client.dio.post('/users', data: {
      'email': email,
      'password': password,
      'name': name,
      'surname': surname,
      'locale': locale,
    });
    return LoginResult.fromJson(_map(response.data));
  }

  Future<User> me() async => User.fromJson(_map((await client.dio.get('/users/me')).data));

  Future<User> updateProfile(Map<String, dynamic> payload) async =>
      User.fromJson(_map((await client.dio.put('/users/me', data: payload)).data));

  Future<User> updatePicture(String picture) async => User.fromJson(
      _map((await client.dio.put('/users/me/picture', data: {'picture': picture, 'x': 50, 'y': 50})).data));

  Future<void> forgotPassword(String email) =>
      client.dio.post('/users/forgot-password', data: {'email': email});

  /// Finishes an MFA-gated login with an authenticator-app code. Security keys
  /// are web-only: the WebAuthn ceremony needs a browser, so the app offers TOTP
  /// and tells the user to use the web app for their key.
  Future<LoginResult> loginTotp(String challengeToken, String code) async {
    final response = await client.dio.post('/users/login/mfa/totp', data: {
      'challengeToken': challengeToken,
      'code': code,
    });
    return LoginResult.fromJson(_map(response.data));
  }

  // --- exercises & 1RMs ---

  Future<List<Exercise>> exercises() async =>
      _list((await client.dio.get('/exercises')).data).map(Exercise.fromJson).toList();

  Future<List<OneRm>> oneRms() async =>
      _list((await client.dio.get('/one-rms')).data).map(OneRm.fromJson).toList();

  Future<OneRm> setOneRm(String exerciseId, double value) async =>
      OneRm.fromJson(_map((await client.dio.put('/one-rms/$exerciseId', data: {'value': value})).data));

  Future<void> deleteOneRm(String exerciseId) => client.dio.delete('/one-rms/$exerciseId');

  // --- clubs ---

  Future<List<Club>> clubs() async =>
      _list((await client.dio.get('/clubs')).data).map(Club.fromJson).toList();

  // --- training ---

  Future<List<Assignment>> assignments() async =>
      _list((await client.dio.get('/training')).data).map(Assignment.fromJson).toList();

  Future<AssignmentDetail> assignment(String assignmentId) async =>
      AssignmentDetail.fromJson(_map((await client.dio.get('/training/$assignmentId')).data));

  Future<SetLog> logSet(
    String assignmentId,
    String setId, {
    int? actualReps,
    double? actualRpe,
    double? actualLoad,
    String comment = '',
    bool done = true,
  }) async {
    final response = await client.dio.put('/training/$assignmentId/sets/$setId/log', data: {
      'actualReps': actualReps,
      'actualRpe': actualRpe,
      'actualLoad': actualLoad,
      'comment': comment,
      'done': done,
    });
    return SetLog.fromJson(_map(response.data));
  }

  Future<void> clearLog(String assignmentId, String setId) =>
      client.dio.delete('/training/$assignmentId/sets/$setId/log');

  Future<Assignment> setAssignmentStatus(String assignmentId, String status) async => Assignment.fromJson(
      _map((await client.dio.put('/training/$assignmentId/status', data: {'status': status})).data));

  // --- social ---

  Future<Page<Post>> feed({int page = 0, bool discover = false}) async {
    final response = await client.dio.get(
      discover ? '/posts/discover' : '/posts',
      queryParameters: {'page': page, 'size': 20},
    );
    return Page.fromJson(_map(response.data), Post.fromJson);
  }

  Future<Post> createPost({
    required String content,
    List<String> pictures = const [],
    List<String> links = const [],
    String visibility = 'public',
    String clubId = '',
  }) async {
    final response = await client.dio.post('/posts', data: {
      'content': content,
      'pictures': pictures,
      'links': links,
      'visibility': visibility,
      'clubId': clubId,
    });
    return Post.fromJson(_map(response.data));
  }

  Future<Post> like(String postId, bool liked) async {
    final response =
        liked ? await client.dio.delete('/posts/$postId/like') : await client.dio.post('/posts/$postId/like');
    return Post.fromJson(_map(response.data));
  }

  Future<void> deletePost(String postId) => client.dio.delete('/posts/$postId');

  Future<Page<Comment>> comments(String postId, {int page = 0}) async {
    final response =
        await client.dio.get('/posts/$postId/comments', queryParameters: {'page': page, 'size': 20});
    return Page.fromJson(_map(response.data), Comment.fromJson);
  }

  Future<Comment> addComment(String postId, String content) async => Comment.fromJson(
      _map((await client.dio.post('/posts/$postId/comments', data: {'content': content})).data));

  Future<void> deleteComment(String postId, String commentId) =>
      client.dio.delete('/posts/$postId/comments/$commentId');

  Future<void> report({
    required String targetType,
    required String targetId,
    required String reason,
    String comment = '',
  }) =>
      client.dio.post('/reports', data: {
        'targetType': targetType,
        'targetId': targetId,
        'reason': reason,
        'comment': comment,
      });

  // --- profiles ---

  Future<PublicProfile> profile(String handle) async =>
      PublicProfile.fromJson(_map((await client.dio.get('/profiles/$handle')).data));

  Future<Page<Post>> profilePosts(String handle, {int page = 0}) async {
    final response =
        await client.dio.get('/profiles/$handle/posts', queryParameters: {'page': page, 'size': 20});
    return Page.fromJson(_map(response.data), Post.fromJson);
  }

  Future<void> follow(String handle, bool following) => following
      ? client.dio.delete('/profiles/$handle/follow')
      : client.dio.post('/profiles/$handle/follow');
}
