package com.example

import org.junit.Test
import io.mockk.MockK
import io.mockk.every
import io.mockk.verify
import io.mockk.mockk

class DoublesTest {
    @MockK
    lateinit var repo: UserRepository

    @MockK
    lateinit var api: ApiClient

    private val fakeDb = FakeDatabase()

    private val testRepository = TestUserRepository()

    private val stubAuth = StubAuthenticator()

    @Test
    fun `repo is setup only`() {
        every { repo.getUser() } returns testUser
        val result = viewModel.loadUser()
        assertThat(result).isEqualTo(testUser)
    }

    @Test
    fun `api is stubbed and verified`() {
        every { api.fetch() } returns data
        sut.process()
        verify { api.fetch() }
    }

    @Test
    fun `uses fake db`() {
        fakeDb.insert(testData)
        val result = sut.query()
        assertThat(result).hasSize(1)
    }

    @Test
    fun `uses test repository`() {
        testRepository.add(testUser)
        val result = sut.findUser()
        assertThat(result).isEqualTo(testUser)
    }

    @Test
    fun `uses stub auth`() {
        stubAuth.setToken("abc")
        val result = sut.authenticate()
        assertThat(result).isTrue()
    }

    @Test
    fun `inline mock creation`() {
        val logger = mockk<Logger>()
        sut.doThing()
        verify { logger.log(any()) }
    }

    @Test
    fun `inline fake creation`() {
        val testClock = TestClock()
        val result = sut.getTime(testClock)
        assertThat(result).isNotNull()
    }
}
